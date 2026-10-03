// Package version содержит метаданные версии сборки.
//
// Значения переменных заполняются на этапе компиляции через -ldflags:
//
//	go build -ldflags "-X github.com/kirsrus/lan2netbox/internal/version.Version=v1.2.3 \
//	 -X github.com/kirsrus/lan2netbox/internal/version.Commit=abc1234"
package version

import (
	"fmt"
	"runtime"
	"strings"
)

// Переменные версии (заполняются через -ldflags из последнего git-тега).
var (
	// Version — семантическая версия из последнего git-тега.
	Version = "dev"
	// Commit — короткий хеш git-коммита.
	Commit = "none"
	// BuildDate — дата сборки (RFC3339).
	BuildDate = "unknown"
)

// Short возвращает краткую строку версии.
func Short() string {
	return Version
}

// String возвращает однострочное описание версии.
func String() string {
	return fmt.Sprintf("%s (commit: %s, built: %s, %s)",
		Version, Commit, BuildDate, runtime.Version())
}

// Details возвращает многострочное описание версии для команды `version`.
func Details() string {
	var b strings.Builder
	b.WriteString("lan2netbox\n")
	fmt.Fprintf(&b, "  version:    %s\n", Version)
	fmt.Fprintf(&b, "  commit:     %s\n", Commit)
	fmt.Fprintf(&b, "  build date: %s\n", BuildDate)
	fmt.Fprintf(&b, "  go:         %s\n", runtime.Version())
	fmt.Fprintf(&b, "  platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return b.String()
}
