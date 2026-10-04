package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in   string
		want slog.Level
	}{
		{"", slog.LevelInfo},
		{"debug", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"Warn", slog.LevelWarn},
		{"error", slog.LevelError},
	}
	for _, tt := range tests {
		got, err := ParseLevel(tt.in)
		if err != nil {
			t.Errorf("ParseLevel(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}

	if _, err := ParseLevel("verbose"); err == nil {
		t.Error("ParseLevel(verbose): ожидалась ошибка")
	}
}

func TestNewJSON(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(Options{Format: FormatJSON, Writer: &buf})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Info("привет", "key", "value", "n", 42)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("невалидный JSON: %v\n%s", err, buf.String())
	}
	if m["msg"] != "привет" || m["key"] != "value" {
		t.Errorf("JSON = %v", m)
	}
	if m["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", m["level"])
	}
	// Место вызова лога должно быть в конце записи.
	src, ok := m["source"].(string)
	if !ok || !strings.Contains(src, ".go:") {
		t.Errorf("source = %v, want строку вида dir/file.go:N", m["source"])
	}
}

func TestNewText(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(Options{Format: FormatText, Writer: &buf})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Info("сообщение", "key", "value")

	out := buf.String()
	for _, want := range []string{"INFO", "сообщение", "key=value"} {
		if !strings.Contains(out, want) {
			t.Errorf("текст не содержит %q:\n%s", want, out)
		}
	}
	// В не-терминал ANSI-коды выводиться не должны.
	if strings.Contains(out, "\x1b[") {
		t.Errorf("в не-терминал попали ANSI-коды: %q", out)
	}
	// Место вызова лога — в конце строки: source=директория/файл:строка.
	sourceRe := regexp.MustCompile(`source=\S+\.go:\d+\s*$`)
	if !sourceRe.MatchString(out) {
		t.Errorf("source отсутствует в конце строки:\n%s", out)
	}
}

func TestNewUnknownFormat(t *testing.T) {
	if _, err := New(Options{Format: "yaml", Writer: &bytes.Buffer{}}); err == nil {
		t.Error("New(format=yaml): ожидалась ошибка")
	}
}

func TestLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	logger, err := New(Options{Format: FormatJSON, Writer: &buf, Level: slog.LevelWarn})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	logger.Debug("debug-msg")
	logger.Warn("warn-msg")

	if strings.Contains(buf.String(), "debug-msg") {
		t.Errorf("debug-запись не должна попасть в лог:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "warn-msg") {
		t.Errorf("warn-запись должна быть в логе:\n%s", buf.String())
	}
}

func TestDetectFormat(t *testing.T) {
	var buf bytes.Buffer

	t.Setenv("LOG_FORMAT", "text")
	if got := DetectFormat(&buf); got != FormatText {
		t.Errorf("LOG_FORMAT=text: DetectFormat = %q, want text", got)
	}

	t.Setenv("LOG_FORMAT", "json")
	if got := DetectFormat(&buf); got != FormatJSON {
		t.Errorf("LOG_FORMAT=json: DetectFormat = %q, want json", got)
	}

	// Не-терминал, не systemd → json.
	t.Setenv("LOG_FORMAT", "")
	if got := DetectFormat(&buf); got != FormatJSON {
		t.Errorf("не-терминал: DetectFormat = %q, want json", got)
	}

	// Окружение systemd → json.
	t.Setenv("INVOCATION_ID", "abc")
	if got := DetectFormat(&buf); got != FormatJSON {
		t.Errorf("systemd: DetectFormat = %q, want json", got)
	}
}

func TestSetupFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	logger, closeFn, err := Setup("debug", "json", path)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	logger.Info("в файл")
	closeFn()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("чтение файла: %v", err)
	}
	if !strings.Contains(string(data), "в файл") {
		t.Errorf("в файле нет записи:\n%s", data)
	}
}

func TestSetupBadLevel(t *testing.T) {
	if _, _, err := Setup("verbose", "json", ""); err == nil {
		t.Error("Setup(verbose): ожидалась ошибка уровня")
	}
}
