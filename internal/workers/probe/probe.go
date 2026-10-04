// Package probe — горутина опроса устройств.
//
// Probe подписан на события device.discovered: новые устройства попадают
// в очередь опроса плагинами (SNMP/HTTP/NETCONF). Публикует
// device.updated и link.changed. Логика опроса пока заглушечная.
package probe

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/kirsrus/lan2netbox/internal/bus"
	"github.com/kirsrus/lan2netbox/internal/config"
)

// Probe — компонент опроса сетевых устройств (плагины SNMP/HTTP/NETCONF).
type Probe struct {
	logger *slog.Logger
	cfg    config.ProbeConfig
	// bus — шина событий: подписка на device.discovered, публикация
	// device.updated и link.changed.
	bus *bus.Bus
}

// New создаёт компонент probe.
func New(logger *slog.Logger, cfg config.ProbeConfig, bus *bus.Bus) *Probe {
	return &Probe{logger: logger, cfg: cfg, bus: bus}
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

	// Подписка на события шины: обнаруженные устройства будут ставиться
	// в очередь опроса. Обработка пока заглушечная — только логирование.
	if err := p.bus.ConsumeTopics(ctx, []bus.Topic{bus.TopicDeviceDiscovered}, p.onDeviceDiscovered); err != nil {
		return fmt.Errorf("probe: подписка на события: %w", err)
	}

	p.logger.Info("probe: остановлен")
	return nil
}

// onDeviceDiscovered обрабатывает событие device.discovered.
// Заглушка: в дальнейшем устройство добавляется в очередь опроса
// плагинами SNMP/HTTP/NETCONF.
func (p *Probe) onDeviceDiscovered(_ context.Context, topic bus.Topic, msg *message.Message) error {
	var ev bus.DeviceDiscoveredEvent
	if err := bus.DecodePayload(msg, &ev); err != nil {
		return err
	}
	p.logger.Info("probe: получено событие",
		"topic", topic.String(),
		"mac", ev.MAC,
		"ip", ev.IP,
		"source", ev.Source,
	)
	return nil
}
