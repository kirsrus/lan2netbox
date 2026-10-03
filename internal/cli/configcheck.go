package cli

import (
	"github.com/spf13/cobra"
)

// newConfigCheckCmd возвращает команду проверки файла конфигурации.
func newConfigCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config-check",
		Short: "Проверка файла конфигурации",
		Long: `Загружает и проверяет YAML-конфигурацию (см. --config),
выводит обнаруженные ошибки и завершается с ненулевым кодом при их наличии.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return notImplemented("config-check")
		},
	}
}
