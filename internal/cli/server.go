package cli

import (
	"github.com/spf13/cobra"
)

// newServerCmd возвращает команду запуска только web-сервера.
func newServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Запуск только web-сервера",
		Long: `Запускает web-интерфейс (gin) без рабочих горутин sniffer/probe/netbox.
Используется для просмотра данных локальной БД.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return notImplemented("server")
		},
	}
}
