package cli

import (
	"fmt"

	"github.com/kirsrus/lan2netbox/internal/config"
	"github.com/spf13/cobra"
)

// newConfigCheckCmd возвращает команду проверки файла конфигурации.
func newConfigCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config-check",
		Short: "Проверка файла конфигурации",
		Long: `Загружает и проверяет YAML-конфигурацию (см. --config),
выводит обнаруженные ошибки и завершается с ненулевым кодом при их наличии.`,
		Args: cobra.NoArgs,
		RunE: configCheck,
	}
}

// configCheck — загрузка и проверка конфигурации.
func configCheck(cmd *cobra.Command, _ []string) error {
	path, err := cmd.Flags().GetString("config")
	if err != nil {
		return err
	}
	if path == "" {
		path = config.DefaultPath
	}

	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("конфигурация %s невалидна: %w", path, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "конфигурация %s корректна\n", path)
	fmt.Fprintf(cmd.OutOrStdout(), "  store.path:       %s\n", cfg.Store.Path)
	fmt.Fprintf(cmd.OutOrStdout(), "  sniffer.interface: %s\n", cfg.Sniffer.Interface)
	fmt.Fprintf(cmd.OutOrStdout(), "  sniffer.subnets:  %v\n", cfg.Sniffer.Subnets)
	fmt.Fprintf(cmd.OutOrStdout(), "  netbox.enabled:   %t\n", cfg.NetBox.Enabled)
	fmt.Fprintf(cmd.OutOrStdout(), "  netbox.url:       %s\n", cfg.NetBox.URL)
	fmt.Fprintf(cmd.OutOrStdout(), "  web.listen:       %s\n", cfg.Web.Listen)
	fmt.Fprintf(cmd.OutOrStdout(), "  logging:          %s/%s\n", cfg.Logging.Level, cfg.Logging.Format)
	return nil
}
