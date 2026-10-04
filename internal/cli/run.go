package cli

import (
	"runtime"

	"github.com/kirsrus/lan2netbox/internal/version"
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
		RunE: run,
	}
}

// run — запуск приложения.
//
// TODO: реализовать на этапе internal/app (оркестратор горутин, graceful shutdown).
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

	logger.Warn("команда \"run\" ещё не реализована")

	return nil
}

// flagString возвращает значение строкового флага команды.
func flagString(cmd *cobra.Command, name string) string {
	v, err := cmd.Flags().GetString(name)
	if err != nil {
		return ""
	}
	return v
}
