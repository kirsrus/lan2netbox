// Package sniffer — горутина обнаружения сетевых устройств.
//
// Sniffer — издатель событий шины: публикует device.discovered,
// device.updated, device.deleted и device.saved. Обнаруженные
// MAC-адреса и связи MAC↔IP накапливаются в локальном кэше и
// периодически сохраняются в БД SQLite. Захват пакетов выполняется
// через github.com/mdlayher/packet (AF_PACKET, Linux, без cgo).
// Обработчики пакетов (ARP, DHCP, mDNS) регистрируются в компоненте
// и вызываются для каждого захваченного кадра.
package sniffer

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kirsrus/lan2netbox/internal/bus"
	"github.com/kirsrus/lan2netbox/internal/config"
)

// Sniffer — компонент обнаружения устройств в LAN (ARP/DHCP/mDNS).
type Sniffer struct {
	logger *slog.Logger
	cfg    config.SnifferConfig
	// bus — шина событий; используется для публикации найденных устройств.
	bus *bus.Bus
	// db — локальная база данных SQLite; используется для загрузки
	// кэша устройств при старте и периодического сохранения изменений.
	db *sql.DB
	// handlers — зарегистрированные обработчики пакетов (ARP, DHCP, mDNS).
	handlers []PacketHandler
}

// New создаёт компонент sniffer.
func New(logger *slog.Logger, cfg config.SnifferConfig, bus *bus.Bus, db *sql.DB) *Sniffer {
	return &Sniffer{logger: logger, cfg: cfg, bus: bus, db: db}
}

// Name возвращает имя компонента для оркестратора.
func (s *Sniffer) Name() string { return "sniffer" }

// RegisterHandler регистрирует обработчик пакетов.
//
// Позволяет расширять сбор данных новыми протоколами (DHCP, mDNS и т.д.)
// без изменения цикла захвата. Обработчики, зарегистрированные этим
// методом, имеют приоритет над конфигурацией capture.
func (s *Sniffer) RegisterHandler(h PacketHandler) {
	s.handlers = append(s.handlers, h)
}

// Run запускает компонент и блокируется до отмены ctx.
//
// При старте загружает кэш обнаруженных устройств (таблицы mac и
// mac_ip) из локальной БД SQLite. Определяет интерфейс захвата,
// создаёт AF_PACKET-сокет (Linux, без cgo, github.com/mdlayher/packet)
// и запускает цикл чтения пакетов с передачей их зарегистрированным
// обработчикам. Параллельно работает горутина синхронизации: раз в
// FlushInterval изменённые записи кэша сохраняются в БД, а id
// сохранённых MAC-записей публикуются в шину (топик device.saved).
func (s *Sniffer) Run(ctx context.Context) error {
	// Определяем интерфейс захвата (из конфигурации или автоопределение).
	iface, err := resolveInterface(s.cfg.Interface)
	if err != nil {
		return err
	}

	// Загружаем кэш обнаруженных устройств из локальной БД SQLite.
	cache := newDeviceCache(s.db)
	if err := cache.Load(); err != nil {
		return fmt.Errorf("sniffer: загрузка кэша устройств: %w", err)
	}
	s.logger.Info("sniffer: кэш устройств загружен",
		"macs", len(cache.macs),
		"mac_ips", len(cache.macIPs),
	)

	// Обработчики: явно зарегистрированные или из конфигурации capture.
	handlers := s.handlers
	if len(handlers) == 0 {
		handlers = s.newHandlers(cache)
	}
	if len(handlers) == 0 {
		s.logger.Warn("sniffer: не зарегистрировано ни одного обработчика пакетов",
			"capture", s.cfg.Capture,
		)
	}

	s.logger.Info("sniffer: запуск",
		"interface", iface,
		"subnets", s.cfg.Subnets,
		"scan_interval", s.cfg.ScanInterval,
		"offline_timeout", s.cfg.OfflineTimeout,
		"flush_interval", s.cfg.FlushInterval,
		"capture", s.cfg.Capture,
		"handlers", handlerNames(handlers),
		"publishes", []string{
			bus.TopicDeviceDiscovered.String(),
			bus.TopicDeviceUpdated.String(),
			bus.TopicDeviceDeleted.String(),
			bus.TopicDeviceSaved.String(),
		},
	)

	// Создаём AF_PACKET-сокет (mdlayher/packet).
	capture, err := newPacketCapture(s.logger, iface)
	if err != nil {
		return err
	}

	// Дочерний контекст: при завершении Run останавливаем горутину
	// синхронизации кэша.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Горутина периодической синхронизации кэша с БД и публикации
	// событий device.saved.
	syncErr := make(chan error, 1)
	go func() {
		if err := s.syncLoop(ctx, cache); err != nil {
			syncErr <- err
		}
	}()

	// Цикл захвата пакетов (блокирующий).
	captureErr := make(chan error, 1)
	go func() {
		captureErr <- capture.run(ctx, handlers)
	}()

	// Ожидаем завершения цикла захвата либо ошибки синхронизации.
	select {
	case err := <-captureErr:
		if err != nil {
			return fmt.Errorf("sniffer: захват пакетов: %w", err)
		}
	case err := <-syncErr:
		return err
	}

	s.logger.Info("sniffer: остановлен")
	return nil
}

// syncLoop периодически (раз в FlushInterval) сохраняет изменённые
// записи кэша в БД и публикует события device.saved. Блокируется до
// отмены ctx либо ошибки сохранения/публикации.
func (s *Sniffer) syncLoop(ctx context.Context, cache *DeviceCache) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(s.cfg.FlushInterval):
			if err := s.flushCache(ctx, cache); err != nil {
				return err
			}
		}
	}
}

// flushCache сохраняет изменённые записи кэша в БД и публикует в шину
// событие device.saved для каждой сохранённой записи MAC-адреса.
//
// Пока подписчиков на топик нет — публикация является заглушкой для
// будущих потребителей (web, netbox и т.п.).
func (s *Sniffer) flushCache(ctx context.Context, cache *DeviceCache) error {
	saved, err := cache.Flush()
	if err != nil {
		return err
	}
	if len(saved) == 0 {
		return nil
	}

	s.logger.Info("sniffer: кэш синхронизирован с БД",
		"saved_macs", len(saved),
	)
	for _, rec := range saved {
		ev := bus.DeviceSavedEvent{
			ID:        rec.id,
			MAC:       rec.mac,
			Timestamp: time.Now().UTC(),
		}
		if err := s.bus.Publish(ctx, bus.TopicDeviceSaved, ev); err != nil {
			return fmt.Errorf("sniffer: публикация события %s: %w", bus.TopicDeviceSaved, err)
		}
	}
	return nil
}

// newHandlers создаёт обработчики пакетов согласно конфигурации capture.
//
// Сейчас реализован только ARP; DHCP и mDNS будут добавлены позже —
// для них логируется предупреждение, чтобы конфигурация не «молчала».
// Обработчики получают кэш устройств для записи обнаруженных MAC/IP.
func (s *Sniffer) newHandlers(cache *DeviceCache) []PacketHandler {
	var handlers []PacketHandler
	for _, name := range s.cfg.Capture {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "arp":
			handlers = append(handlers, NewARPHandler(s.logger, cache))
		case "dhcp", "mdns":
			s.logger.Warn("sniffer: обработчик ещё не реализован", "protocol", name)
		default:
			s.logger.Warn("sniffer: неизвестный протокол захвата", "protocol", name)
		}
	}
	return handlers
}

// handlerNames возвращает имена обработчиков для логов.
func handlerNames(handlers []PacketHandler) []string {
	names := make([]string, 0, len(handlers))
	for _, h := range handlers {
		names = append(names, h.Name())
	}
	return names
}
