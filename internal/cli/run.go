package cli

import (
	"context"
	"fmt"
	"runtime"

	"github.com/kirsrus/lan2netbox/internal/app"
	"github.com/kirsrus/lan2netbox/internal/bus"
	"github.com/kirsrus/lan2netbox/internal/config"
	"github.com/kirsrus/lan2netbox/internal/version"
	"github.com/kirsrus/lan2netbox/internal/workers/netbox"
	"github.com/kirsrus/lan2netbox/internal/workers/probe"
	"github.com/kirsrus/lan2netbox/internal/workers/sniffer"
	"github.com/kirsrus/lan2netbox/internal/workers/web"
	"github.com/kirsrus/lan2netbox/internal/workers/zabbix"
	"github.com/kirsrus/lan2netbox/pkg/logging"
	"github.com/spf13/cobra"
)

// newRunCmd возвращает команду запуска основного приложения.
func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Запуск основного приложения",
		Long: `Запускает все рабочие компоненты: sniffer, probe, netbox, web, zabbix.
Приложение работает до получения сигнала завершения (SIGINT/SIGTERM).`,
		Args: cobra.NoArgs,
		RunE: run,
	}
}

// run — запуск приложения: инициализация логирования, загрузка
// конфигурации, сборка рабочих компонентов и запуск оркестратора
// internal/app.
func run(cmd *cobra.Command, _ []string) error {
	logger, closeFn, err := logging.Setup(
		flagString(cmd, "log-level"),
		flagString(cmd, "log-format"),
		flagString(cmd, "log-file"),
	)
	if err != nil {
		return err
	}
	defer closeFn()

	// Индикация начала логирования с информацией о сборке.
	logger.Info("запуск логирования",
		"version", version.Short(),
		"commit", version.Commit,
		"build_date", version.BuildDate,
		"go", runtime.Version(),
		"platform", runtime.GOOS+"/"+runtime.GOARCH,
	)

	// Загрузка конфигурации. При отсутствии файла используются
	// значения по умолчанию — приложение запускается «из коробки».
	path := flagString(cmd, "config")
	if path == "" {
		path = config.DefaultPath
	}
	cfg, usedDefaults, err := config.LoadOrDefault(path)
	if err != nil {
		return fmt.Errorf("загрузка конфигурации: %w", err)
	}
	if usedDefaults {
		logger.Warn("файл конфигурации не найден, используются значения по умолчанию",
			"path", path)
	} else {
		logger.Info("конфигурация загружена", "path", path)
	}

	// Инициализация внутренней шины событий (watermill gochannel).
	// Компоненты обмениваются событиями только через неё.
	eventBus := bus.New(logger, bus.DefaultConfig())
	defer eventBus.Close()
	logger.Info("шина событий инициализирована",
		"transport", "watermill/gochannel",
		"topics", len(bus.AllTopics()),
	)

	// Сборка рабочих компонентов. Компоненты с enabled=false не запускаются.
	// Каждый воркер получает ссылку на шину для публикации/подписки.
	workers := []app.Worker{
		sniffer.New(logger, cfg.Sniffer, eventBus),
		probe.New(logger, cfg.Probe, eventBus),
	}
	if cfg.NetBox.Enabled {
		workers = append(workers, netbox.New(logger, cfg.NetBox, eventBus))
	}
	workers = append(workers, web.New(logger, cfg.Web, eventBus))
	if cfg.Zabbix.Enabled {
		workers = append(workers, zabbix.New(logger, cfg.Zabbix, eventBus))
	}

	// Запуск оркестратора. Блокируется до SIGINT/SIGTERM либо ошибки компонента.
	return app.New(logger, workers...).Run(context.Background())
}

// flagString возвращает значение строкового флага команды.
func flagString(cmd *cobra.Command, name string) string {
	v, err := cmd.Flags().GetString(name)
	if err != nil {
		return ""
	}
	return v
}
