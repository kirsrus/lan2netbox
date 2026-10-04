// Package netbox — горутина синхронизации с NetBox.
//
// NetBox подписан на события device.updated, device.deleted, link.changed
// и sync.request. Обработка пока заглушечная — события логируются.
package netbox

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kirsrus/lan2netbox/internal/bus"
	"github.com/kirsrus/lan2netbox/internal/config"
)

// NetBox — компонент синхронизации устройств с NetBox.
type NetBox struct {
	logger *slog.Logger
	cfg    config.NetBoxConfig
	// bus — шина событий: подписка на изменения устройств/связей.
	bus *bus.Bus
}

// New создаёт компонент netbox.
func New(logger *slog.Logger, cfg config.NetBoxConfig, bus *bus.Bus) *NetBox {
	return &NetBox{logger: logger, cfg: cfg, bus: bus}
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

	// Подписка на события шины (заглушка: логирование). В дальнейшем —
	// поиск устройства в NetBox по MAC, автосоздание устройств с ролями/
	// типами, интерфейсами и кабелями, маппинг локальный MAC ↔ NetBox ID.
	if err := n.bus.ConsumeTopics(ctx, []bus.Topic{
		bus.TopicDeviceUpdated,
		bus.TopicDeviceDeleted,
		bus.TopicLinkChanged,
		bus.TopicSyncRequest,
	}, n.bus.LogHandler("netbox")); err != nil {
		return fmt.Errorf("netbox: подписка на события: %w", err)
	}

	n.logger.Info("netbox: остановлен")
	return nil
}
