// Package web — горутина web-интерфейса (gin + SSE).
//
// Заглушка: регистрируется в оркестраторе internal/app, запускается,
// ждёт отмены контекста и завершает работу.
package web

import (
	"context"
	"log/slog"

	"github.com/kirsrus/lan2netbox/internal/config"
)

// Web — компонент web-интерфейса.
type Web struct {
	logger *slog.Logger
	cfg    config.WebConfig
}

// New создаёт компонент web.
func New(logger *slog.Logger, cfg config.WebConfig) *Web {
	return &Web{logger: logger, cfg: cfg}
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

	<-ctx.Done()
	w.logger.Info("web: остановлен")
	return nil
}
