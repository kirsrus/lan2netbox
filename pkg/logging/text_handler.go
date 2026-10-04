package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// ANSI-цвета (применяются только при выводе в терминал).
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiBlue   = "\x1b[34m"
	ansiGray   = "\x1b[90m"
)

// bufferPool — переиспользование буферов форматирования.
var bufferPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// textHandler — кастомный человекочитаемый handler для slog.
type textHandler struct {
	opts   *slog.HandlerOptions
	groups []string
	attrs  []slog.Attr

	mu    *sync.Mutex
	out   io.Writer
	color bool
}

// newTextHandler создаёт текстовый handler; ANSI-цвета включаются
// только при выводе в терминал.
func newTextHandler(out io.Writer, opts *slog.HandlerOptions) *textHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	h := &textHandler{
		opts: opts,
		mu:   &sync.Mutex{},
		out:  out,
	}
	if f, ok := out.(*os.File); ok {
		h.color = term.IsTerminal(int(f.Fd()))
	}
	return h
}

// Enabled сообщает, нужно ли обрабатывать записи уровня level.
func (h *textHandler) Enabled(_ context.Context, level slog.Level) bool {
	if h.opts.Level == nil {
		return true
	}
	return level >= h.opts.Level.Level()
}

// Handle форматирует и выводит одну запись лога.
func (h *textHandler) Handle(_ context.Context, r slog.Record) error {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	// Время.
	h.writeColor(buf, ansiGray)
	buf.WriteString(r.Time.Format("2006-01-02 15:04:05.000"))
	h.writeColor(buf, ansiReset)
	buf.WriteByte(' ')

	// Уровень (с выравниванием до 5 символов).
	h.writeColor(buf, levelColor(r.Level))
	level := r.Level.String()
	if len(level) < 5 {
		level += strings.Repeat(" ", 5-len(level))
	}
	buf.WriteString(level)
	h.writeColor(buf, ansiReset)
	buf.WriteByte(' ')

	// Сообщение.
	buf.WriteString(r.Message)

	// Постоянные атрибуты и атрибуты записи.
	prefix := strings.Join(h.groups, ".")
	h.appendAttrs(buf, prefix, h.attrs)

	attrs := make([]slog.Attr, 0, 8)
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})
	h.appendAttrs(buf, prefix, attrs)

	buf.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf.Bytes())
	return err
}

// WithAttrs возвращает копию handler'а с постоянными атрибутами.
func (h *textHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	h2 := *h
	h2.attrs = append(cloneAttrs(h.attrs), attrs...)
	return &h2
}

// WithGroup возвращает копию handler'а с именованной группой.
func (h *textHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := *h
	h2.groups = append(cloneGroups(h.groups), name)
	return &h2
}

// appendAttrs добавляет атрибуты в формате key=value.
func (h *textHandler) appendAttrs(buf *bytes.Buffer, prefix string, attrs []slog.Attr) {
	for _, a := range attrs {
		if a.Equal(slog.Attr{}) {
			continue
		}
		key := a.Key
		if prefix != "" {
			key = prefix + "." + a.Key
		}
		if a.Value.Kind() == slog.KindGroup {
			h.appendAttrs(buf, key, a.Value.Group())
			continue
		}
		buf.WriteByte(' ')
		buf.WriteString(key)
		buf.WriteByte('=')
		h.appendValue(buf, a.Value)
	}
}

// appendValue добавляет значение атрибута в текстовом виде.
func (h *textHandler) appendValue(buf *bytes.Buffer, v slog.Value) {
	switch v.Kind() {
	case slog.KindString:
		writeString(buf, v.String())
	case slog.KindInt64:
		buf.WriteString(strconv.FormatInt(v.Int64(), 10))
	case slog.KindUint64:
		buf.WriteString(strconv.FormatUint(v.Uint64(), 10))
	case slog.KindFloat64:
		buf.WriteString(strconv.FormatFloat(v.Float64(), 'g', -1, 64))
	case slog.KindBool:
		buf.WriteString(strconv.FormatBool(v.Bool()))
	case slog.KindTime:
		buf.WriteString(v.Time().Format(time.RFC3339Nano))
	case slog.KindDuration:
		buf.WriteString(v.Duration().String())
	default:
		fmt.Fprint(buf, v.Any())
	}
}

// writeString выводит строку; при наличии пробелов/спецсимволов —
// в кавычках.
func writeString(buf *bytes.Buffer, s string) {
	if strings.ContainsAny(s, " \t\"=") {
		buf.WriteString(strconv.Quote(s))
		return
	}
	buf.WriteString(s)
}

// levelColor возвращает ANSI-цвет для уровня лога.
func levelColor(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return ansiRed + ansiBold
	case l >= slog.LevelWarn:
		return ansiYellow
	case l >= slog.LevelInfo:
		return ansiGreen
	default:
		return ansiGray
	}
}

// writeColor добавляет ANSI-код, только если включена раскраска.
func (h *textHandler) writeColor(buf *bytes.Buffer, color string) {
	if h.color {
		buf.WriteString(color)
	}
}

// cloneAttrs и cloneGroups — безопасное копирование слайсов.
func cloneAttrs(attrs []slog.Attr) []slog.Attr {
	if attrs == nil {
		return nil
	}
	out := make([]slog.Attr, len(attrs))
	copy(out, attrs)
	return out
}

func cloneGroups(groups []string) []string {
	if groups == nil {
		return nil
	}
	out := make([]string, len(groups))
	copy(out, groups)
	return out
}
