// Package web — горутина web-интерфейса (gin + SSE).
//
// Web подписан на события device.discovered, device.updated,
// device.deleted и link.changed для трансляции клиентам через
// SSE-канал /events. Обработка пока заглушечная — события логируются.
package web

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kirsrus/lan2netbox/internal/bus"
	"github.com/kirsrus/lan2netbox/internal/config"
)

// Web — компонент web-интерфейса.
type Web struct {
	logger *slog.Logger
	cfg    config.WebConfig
	// bus — шина событий: подписка на события для SSE-трансляции.
	bus *bus.Bus
}

// New создаёт компонент web.
func New(logger *slog.Logger, cfg config.WebConfig, bus *bus.Bus) *Web {
	return &Web{logger: logger, cfg: cfg, bus: bus}
}

// Name возвращает имя компонента для оркестратора.
func (w *Web) Name() string { return "web" }

// Run запускает компонент и блокируется до отмены ctx.
func (w *Web) Run(ctx context.Context) error {
	w.logger.Info("web: запуск (заглушка)",
		"listen", w.cfg.Listen,
		"templates_dir", w.cfg.TemplatesDir,
		"static_dir", w.cfg.StaticDir,
	)

	// Подписка на события шины для SSE-канала /events (заглушка:
	// логирование). Публикация sync.request при ручном запуске
	// синхронизации будет добавлена вместе с gin-эндпоинтами.
	if err := w.bus.ConsumeTopics(ctx, []bus.Topic{
		bus.TopicDeviceDiscovered,
		bus.TopicDeviceUpdated,
		bus.TopicDeviceDeleted,
		bus.TopicLinkChanged,
	}, w.bus.LogHandler("web")); err != nil {
		return fmt.Errorf("web: подписка на события: %w", err)
	}

	w.logger.Info("web: остановлен")
	return nil
}
