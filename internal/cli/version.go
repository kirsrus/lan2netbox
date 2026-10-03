package cli

import (
	"fmt"

	"github.com/kirsrus/lan2netbox/internal/version"
	"github.com/spf13/cobra"
)

// newVersionCmd возвращает команду вывода версии сборки.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Печать версии сборки",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(cmd.OutOrStdout(), version.Details())
			return nil
		},
	}
}
