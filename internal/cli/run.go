package cli

import (
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
func run(_ *cobra.Command, _ []string) error {
	return notImplemented("run")
}
