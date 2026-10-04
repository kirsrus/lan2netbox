package logging

import (
	"context"
	"log/slog"
	"path/filepath"
	"strconv"
)

// sourceKey — имя атрибута с местом вызова лога.
const sourceKey = "source"

// sourceHandler — декоратор slog.Handler, добавляющий в конец каждой
// записи атрибут source с местом вызова лога:
//
//	<последняя директория>/<имя файла>:<номер строки>
type sourceHandler struct {
	inner slog.Handler
}

// Enabled делегирует внутреннему handler'у.
func (h *sourceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle добавляет атрибут source и делегирует внутреннему handler'у.
func (h *sourceHandler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(slog.String(sourceKey, sourceOf(r)))
	return h.inner.Handle(ctx, r)
}

// WithAttrs сохраняет обёртку.
func (h *sourceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &sourceHandler{inner: h.inner.WithAttrs(attrs)}
}

// WithGroup сохраняет обёртку.
func (h *sourceHandler) WithGroup(name string) slog.Handler {
	return &sourceHandler{inner: h.inner.WithGroup(name)}
}

// sourceOf возвращает место вызова лога: последняя директория/файл:строка.
func sourceOf(r slog.Record) string {
	src := r.Source()
	if src == nil || src.File == "" {
		return "???"
	}
	dir := filepath.Base(filepath.Dir(src.File))
	file := filepath.Base(src.File)
	return filepath.ToSlash(filepath.Join(dir, file)) + ":" + strconv.Itoa(src.Line)
}
