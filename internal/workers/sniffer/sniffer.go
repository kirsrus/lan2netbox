// Package sniffer — горутина обнаружения сетевых устройств.
//
// Sniffer — издатель событий шины: публикует device.discovered,
// device.updated и device.deleted. Захват пакетов выполняется через
// github.com/google/gopacket/afpacket (AF_PACKET, Linux, без cgo).
// Обработчики пакетов (ARP, DHCP, mDNS) регистрируются в компоненте
// и вызываются для каждого захваченного кадра.
package sniffer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kirsrus/lan2netbox/internal/bus"
	"github.com/kirsrus/lan2netbox/internal/config"
)

// Sniffer — компонент обнаружения устройств в LAN (ARP/DHCP/mDNS).
type Sniffer struct {
	logger *slog.Logger
	cfg    config.SnifferConfig
	// bus — шина событий; используется для публикации найденных устройств.
	bus *bus.Bus
	// handlers — зарегистрированные обработчики пакетов (ARP, DHCP, mDNS).
	handlers []PacketHandler
}

// New создаёт компонент sniffer.
func New(logger *slog.Logger, cfg config.SnifferConfig, bus *bus.Bus) *Sniffer {
	return &Sniffer{logger: logger, cfg: cfg, bus: bus}
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
// Определяет интерфейс захвата, создаёт AF_PACKET-сокет (Linux) и
// запускает цикл чтения пакетов с передачей их зарегистрированным
// обработчикам. Публикация device.discovered / device.updated /
// device.deleted будет реализована вместе с обработчиками DHCP/mDNS
// и активным ARP-сканом.
func (s *Sniffer) Run(ctx context.Context) error {
	// Определяем интерфейс захвата (из конфигурации или автоопределение).
	iface, err := resolveInterface(s.cfg.Interface)
	if err != nil {
		return err
	}

	// Обработчики: явно зарегистрированные или из конфигурации capture.
	handlers := s.handlers
	if len(handlers) == 0 {
		handlers = s.newHandlers()
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
		"capture", s.cfg.Capture,
		"handlers", handlerNames(handlers),
		"publishes", []string{
			bus.TopicDeviceDiscovered.String(),
			bus.TopicDeviceUpdated.String(),
			bus.TopicDeviceDeleted.String(),
		},
	)

	// Создаём AF_PACKET-сокет и запускаем цикл захвата.
	capture, err := newPacketCapture(s.logger, iface)
	if err != nil {
		return err
	}
	if err := capture.run(ctx, handlers); err != nil {
		return fmt.Errorf("sniffer: захват пакетов: %w", err)
	}

	s.logger.Info("sniffer: остановлен")
	return nil
}

// newHandlers создаёт обработчики пакетов согласно конфигурации capture.
//
// Сейчас реализован только ARP; DHCP и mDNS будут добавлены позже —
// для них логируется предупреждение, чтобы конфигурация не «молчала».
func (s *Sniffer) newHandlers() []PacketHandler {
	var handlers []PacketHandler
	for _, name := range s.cfg.Capture {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "arp":
			handlers = append(handlers, NewARPHandler(s.logger))
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
