package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newVersionCommand creates the version subcommand
func newVersionCommand(buildInfo *BuildInfo) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("bazinga version %s\n", buildInfo.Version)
			fmt.Printf("  commit: %s\n", buildInfo.Commit)
			fmt.Printf("  built: %s\n", buildInfo.Date)
			return nil
		},
	}

	return cmd
}
