// Package cli определяет дерево команд cobra для lan2netbox.
package cli

import (
	"fmt"
	"os"

	"github.com/kirsrus/lan2netbox/internal/version"
	"github.com/spf13/cobra"
)

// rootCmd — корневая команда. Вызов без подкоманды равнозначен `run`.
var rootCmd = &cobra.Command{
	Use:   "lan2netbox",
	Short: "Обнаружение сетевых устройств и синхронизация с NetBox",
	Long: `lan2netbox — кросс-платформенное консольное приложение для обнаружения
сетевых устройств в LAN, их опроса, хранения данных в локальной БД
и синхронизации с NetBox.`,
	Version:       version.Short(),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          run,
}

// Execute — точка входа для всех команд приложения.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "ошибка:", err)
		os.Exit(1)
	}
}

func init() {
	// Глобальные флаги, общие для всех команд.
	pf := rootCmd.PersistentFlags()
	pf.StringP("config", "c", "", "путь к файлу конфигурации YAML (по умолчанию configs/config.yaml)")
	pf.String("log-level", "info", "уровень логирования: debug | info | warn | error")
	pf.String("log-format", "", "формат логов: text | json (по умолчанию автодетект)")
	pf.String("log-file", "", "путь к файлу лога (по умолчанию stderr)")

	rootCmd.AddCommand(
		newRunCmd(),
		newVersionCmd(),
		newServerCmd(),
		newConfigCheckCmd(),
	)
}
