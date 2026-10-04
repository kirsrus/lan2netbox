// Package netbox — горутина синхронизации с NetBox.
//
// Заглушка: регистрируется в оркестраторе internal/app, запускается,
// ждёт отмены контекста и завершает работу.
package netbox

import (
	"context"
	"log/slog"

	"github.com/kirsrus/lan2netbox/internal/config"
)

// NetBox — компонент синхронизации устройств с NetBox.
type NetBox struct {
	logger *slog.Logger
	cfg    config.NetBoxConfig
}

// New создаёт компонент netbox.
func New(logger *slog.Logger, cfg config.NetBoxConfig) *NetBox {
	return &NetBox{logger: logger, cfg: cfg}
}

// Name возвращает имя компонента для оркестратора.
func (n *NetBox) Name() string { return "netbox" }

// Run запускает компонент и блокируется до отмены ctx.
func (n *NetBox) Run(ctx context.Context) error {
	n.logger.Info("netbox: запуск (заглушка)",
		"url", n.cfg.URL,
		"auto_create", n.cfg.AutoCreate,
		"sync_interval", n.cfg.SyncInterval,
	)

	<-ctx.Done()
	n.logger.Info("netbox: остановлен")
	return nil
}
