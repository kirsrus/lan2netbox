// Package logging предоставляет кастомный slog.Handler для lan2netbox.
//
// Поддерживаются два формата вывода:
//   - text — человекочитаемый цветной вывод в терминал;
//   - json — структурированный вывод (JSON Lines) для journald,
//     файлов и пайпов.
//
// Формат выбирается автоматически (см. DetectFormat) по следующему
// приоритету: переменная окружения LOG_FORMAT → окружение systemd
// (INVOCATION_ID / JOURNAL_STREAM) → наличие TTY у вывода.
//
// В конец каждой записи добавляется место вызова лога
// (последняя директория/имя файла:номер строки).
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/term"
)

// Форматы вывода логов.
const (
	// FormatText — человекочитаемый текст (терминал).
	FormatText = "text"
	// FormatJSON — структурированный JSON (journald, файлы, пайпы).
	FormatJSON = "json"
	// FormatAuto — автоматический выбор формата.
	FormatAuto = "auto"
)

// Options — параметры создания логгера.
type Options struct {
	// Level — минимальный уровень логирования. Нулевое значение —
	// slog.LevelInfo.
	Level slog.Level
	// Format — FormatText, FormatJSON или FormatAuto (пусто = FormatAuto).
	Format string
	// Writer — вывод логов; nil — os.Stderr.
	Writer io.Writer
}

// New создаёт slog.Logger согласно опциям.
func New(opts Options) (*slog.Logger, error) {
	if opts.Writer == nil {
		opts.Writer = os.Stderr
	}

	format := opts.Format
	if format == "" || format == FormatAuto {
		format = DetectFormat(opts.Writer)
	}

	lv := new(slog.LevelVar)
	lv.Set(opts.Level)
	ho := &slog.HandlerOptions{Level: lv}

	var handler slog.Handler
	switch format {
	case FormatText:
		handler = newTextHandler(opts.Writer, ho)
	case FormatJSON:
		handler = slog.NewJSONHandler(opts.Writer, ho)
	default:
		return nil, fmt.Errorf("logging: неизвестный формат %q (допустимо: %s, %s)",
			format, FormatText, FormatJSON)
	}

	// Обёртка добавляет в конец каждой записи место вызова лога.
	return slog.New(&sourceHandler{inner: handler}), nil
}

// Setup — удобный конструктор для инициализации логгера из строковых
// настроек (CLI-флаги или конфигурация).
//
// Возвращает логгер и функцию закрытия ресурсов; функцию нужно вызвать
// при завершении работы, если логи пишутся в файл.
func Setup(level, format, file string) (*slog.Logger, func(), error) {
	lvl, err := ParseLevel(level)
	if err != nil {
		return nil, nil, err
	}

	out := io.Writer(os.Stderr)
	closeFn := func() {}

	if file != "" {
		f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
		if err != nil {
			return nil, nil, fmt.Errorf("logging: открыть файл лога %s: %w", file, err)
		}
		out = f
		closeFn = func() { _ = f.Close() }
	}

	logger, err := New(Options{Level: lvl, Format: format, Writer: out})
	if err != nil {
		closeFn()
		return nil, nil, err
	}
	return logger, closeFn, nil
}

// ParseLevel преобразует строковое представление уровня логирования
// (debug, info, warn, error) в slog.Level. Пустая строка — info.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("logging: неизвестный уровень %q (допустимо: debug, info, warn, error)", s)
	}
}

// DetectFormat определяет формат вывода автоматически.
//
// Приоритет:
//  1. переменная окружения LOG_FORMAT (text | json);
//  2. окружение systemd — INVOCATION_ID или JOURNAL_STREAM → json;
//  3. вывод в терминал → text; иначе (файл, пайп) → json.
func DetectFormat(w io.Writer) string {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT"))); v != "" {
		switch v {
		case FormatText, FormatJSON:
			return v
		}
		// Неизвестное значение игнорируем и продолжаем автодетект.
	}

	if IsSystemdEnvironment() {
		return FormatJSON
	}
	if w == nil || !IsTerminal(w) {
		return FormatJSON
	}
	return FormatText
}

// IsSystemdEnvironment определяет, запущен ли процесс под systemd
// (вывод идёт в journald).
func IsSystemdEnvironment() bool {
	_, invID := os.LookupEnv("INVOCATION_ID")
	_, jStream := os.LookupEnv("JOURNAL_STREAM")
	return invID || jStream
}

// IsTerminal определяет, является ли writer терминалом.
func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
