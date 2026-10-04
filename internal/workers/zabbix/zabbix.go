// Package zabbix — горутина синхронизации с Zabbix.
//
// Заглушка: регистрируется в оркестраторе internal/app, запускается,
// ждёт отмены контекста и завершает работу.
package zabbix

import (
	"context"
	"log/slog"

	"github.com/kirsrus/lan2netbox/internal/config"
)

// Zabbix — компонент синхронизации устройств с Zabbix.
type Zabbix struct {
	logger *slog.Logger
	cfg    config.ZabbixConfig
}

// New создаёт компонент zabbix.
func New(logger *slog.Logger, cfg config.ZabbixConfig) *Zabbix {
	return &Zabbix{logger: logger, cfg: cfg}
}

// Name возвращает имя компонента для оркестратора.
func (z *Zabbix) Name() string { return "zabbix" }

// Run запускает компонент и блокируется до отмены ctx.
func (z *Zabbix) Run(ctx context.Context) error {
	z.logger.Info("zabbix: запуск (заглушка)",
		"url", z.cfg.URL,
	)

	<-ctx.Done()
	z.logger.Info("zabbix: остановлен")
	return nil
}
