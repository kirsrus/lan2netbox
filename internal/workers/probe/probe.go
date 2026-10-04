// Package probe — горутина опроса устройств.
//
// Заглушка: регистрируется в оркестраторе internal/app, запускается,
// ждёт отмены контекста и завершает работу.
package probe

import (
	"context"
	"log/slog"

	"github.com/kirsrus/lan2netbox/internal/config"
)

// Probe — компонент опроса сетевых устройств (плагины SNMP/HTTP/NETCONF).
type Probe struct {
	logger *slog.Logger
	cfg    config.ProbeConfig
}

// New создаёт компонент probe.
func New(logger *slog.Logger, cfg config.ProbeConfig) *Probe {
	return &Probe{logger: logger, cfg: cfg}
}

// Name возвращает имя компонента для оркестратора.
func (p *Probe) Name() string { return "probe" }

// Run запускает компонент и блокируется до отмены ctx.
func (p *Probe) Run(ctx context.Context) error {
	p.logger.Info("probe: запуск (заглушка)",
		"interval", p.cfg.Interval,
		"retry_initial", p.cfg.RetryInitial,
		"retry_max", p.cfg.RetryMax,
		"plugins", map[string]bool{
			"snmp":    p.cfg.Plugins.SNMP.Enabled,
			"http":    p.cfg.Plugins.HTTP.Enabled,
			"netconf": p.cfg.Plugins.NETCONF.Enabled,
		},
	)

	<-ctx.Done()
	p.logger.Info("probe: остановлен")
	return nil
}
