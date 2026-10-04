// Package zabbix — горутина синхронизации с Zabbix.
//
// Zabbix подписан на события device.discovered, device.updated,
// device.deleted и link.changed. Интеграция пока заглушечная —
// события логируются.
package zabbix

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kirsrus/lan2netbox/internal/bus"
	"github.com/kirsrus/lan2netbox/internal/config"
)

// Zabbix — компонент синхронизации устройств с Zabbix.
type Zabbix struct {
	logger *slog.Logger
	cfg    config.ZabbixConfig
	// bus — шина событий: подписка на события для синхронизации хостов.
	bus *bus.Bus
}

// New создаёт компонент zabbix.
func New(logger *slog.Logger, cfg config.ZabbixConfig, bus *bus.Bus) *Zabbix {
	return &Zabbix{logger: logger, cfg: cfg, bus: bus}
}

// Name возвращает имя компонента для оркестратора.
func (z *Zabbix) Name() string { return "zabbix" }

// Run запускает компонент и блокируется до отмены ctx.
func (z *Zabbix) Run(ctx context.Context) error {
	z.logger.Info("zabbix: запуск (заглушка)",
		"url", z.cfg.URL,
	)

	// Подписка на события шины (заглушка: логирование). В дальнейшем —
	// обновление хостов/интерфейсов Zabbix по событиям discovery.
	if err := z.bus.ConsumeTopics(ctx, []bus.Topic{
		bus.TopicDeviceDiscovered,
		bus.TopicDeviceUpdated,
		bus.TopicDeviceDeleted,
		bus.TopicLinkChanged,
	}, z.bus.LogHandler("zabbix")); err != nil {
		return fmt.Errorf("zabbix: подписка на события: %w", err)
	}

	z.logger.Info("zabbix: остановлен")
	return nil
}
