// Package sniffer — горутина обнаружения сетевых устройств.
//
// Sniffer — издатель событий шины: публикует device.discovered,
// device.updated и device.deleted. Логика захвата пока заглушечная:
// компонент запускается, ждёт отмены контекста и завершает работу.
package sniffer

import (
	"context"
	"log/slog"

	"github.com/kirsrus/lan2netbox/internal/bus"
	"github.com/kirsrus/lan2netbox/internal/config"
)

// Sniffer — компонент обнаружения устройств в LAN (ARP/DHCP/mDNS).
type Sniffer struct {
	logger *slog.Logger
	cfg    config.SnifferConfig
	// bus — шина событий; используется для публикации найденных устройств.
	bus *bus.Bus
}

// New создаёт компонент sniffer.
func New(logger *slog.Logger, cfg config.SnifferConfig, bus *bus.Bus) *Sniffer {
	return &Sniffer{logger: logger, cfg: cfg, bus: bus}
}

// Name возвращает имя компонента для оркестратора.
func (s *Sniffer) Name() string { return "sniffer" }

// Run запускает компонент и блокируется до отмены ctx.
//
// Заглушка: публикация device.discovered / device.updated /
// device.deleted будет реализована вместе с логикой захвата пакетов
// (ARP-скан, перехват DHCP/mDNS, контроль времени последней активности).
func (s *Sniffer) Run(ctx context.Context) error {
	s.logger.Info("sniffer: запуск (заглушка)",
		"interface", s.cfg.Interface,
		"subnets", s.cfg.Subnets,
		"scan_interval", s.cfg.ScanInterval,
		"offline_timeout", s.cfg.OfflineTimeout,
		"capture", s.cfg.Capture,
		"publishes", []string{
			bus.TopicDeviceDiscovered.String(),
			bus.TopicDeviceUpdated.String(),
			bus.TopicDeviceDeleted.String(),
		},
	)

	<-ctx.Done()
	s.logger.Info("sniffer: остановлен")
	return nil
}
