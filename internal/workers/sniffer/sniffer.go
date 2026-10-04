// Package sniffer — горутина обнаружения сетевых устройств.
//
// Заглушка: регистрируется в оркестраторе internal/app, запускается,
// ждёт отмены контекста и завершает работу.
package sniffer

import (
	"context"
	"log/slog"

	"github.com/kirsrus/lan2netbox/internal/config"
)

// Sniffer — компонент обнаружения устройств в LAN (ARP/DHCP/mDNS).
type Sniffer struct {
	logger *slog.Logger
	cfg    config.SnifferConfig
}

// New создаёт компонент sniffer.
func New(logger *slog.Logger, cfg config.SnifferConfig) *Sniffer {
	return &Sniffer{logger: logger, cfg: cfg}
}

// Name возвращает имя компонента для оркестратора.
func (s *Sniffer) Name() string { return "sniffer" }

// Run запускает компонент и блокируется до отмены ctx.
func (s *Sniffer) Run(ctx context.Context) error {
	s.logger.Info("sniffer: запуск (заглушка)",
		"interface", s.cfg.Interface,
		"subnets", s.cfg.Subnets,
		"scan_interval", s.cfg.ScanInterval,
		"offline_timeout", s.cfg.OfflineTimeout,
		"capture", s.cfg.Capture,
	)

	<-ctx.Done()
	s.logger.Info("sniffer: остановлен")
	return nil
}
