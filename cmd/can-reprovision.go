package cmd

import (
	"context"
	"fmt"

	"github.com/loft-sh/devpod-provider-kubernetes/pkg/options"
	"github.com/loft-sh/log"
	"github.com/spf13/cobra"
)

// CanReprovisionCmd holds the cmd flags
type CanReprovisionCmd struct{}

// NewRunCmd defines a command
func NewCanReprovisionCmd() *cobra.Command {
	cmd := &CanReprovisionCmd{}
	canReprovisionCmd := &cobra.Command{
		Use:   "can-reprovision",
		Short: "Run a container",
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := options.FromEnv()
			if err != nil {
				return err
			}

			return cmd.Run(context.Background(), options, log.Default)
		},
	}

	return canReprovisionCmd
}

// Run runs the command logic
func (cmd *CanReprovisionCmd) Run(ctx context.Context, options *options.Options, log log.Logger) error {
	if !options.ReprovisioningMode {
		return fmt.Errorf("Reprovisioning disabled")
	}

	return nil
}
