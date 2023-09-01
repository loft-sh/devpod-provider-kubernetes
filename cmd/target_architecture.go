package cmd

import (
	"context"
	"fmt"

	"github.com/loft-sh/devpod-provider-kubernetes/pkg/kubernetes"
	"github.com/loft-sh/devpod-provider-kubernetes/pkg/options"
	"github.com/loft-sh/log"
	"github.com/spf13/cobra"
)

// TargetArchitectureCmd holds the cmd flags
type TargetArchitectureCmd struct{}

// NewTargetArchitectureCmd defines a command
func NewTargetArchitectureCmd() *cobra.Command {
	cmd := &TargetArchitectureCmd{}
	targetArchitectureCmd := &cobra.Command{
		Use:   "target-architecture",
		Short: "TargetArchitecture a container",
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := options.FromEnv()
			if err != nil {
				return err
			}

			return cmd.Run(context.Background(), options, log.Default.ErrorStreamOnly())
		},
	}

	return targetArchitectureCmd
}

// Run runs the command logic
func (cmd *TargetArchitectureCmd) Run(ctx context.Context, options *options.Options, log log.Logger) error {
	arch, err := kubernetes.NewKubernetesDriver(options, log).TargetArchitecture(ctx, options.DevContainerID)
	if err != nil {
		return fmt.Errorf("get target architecture: %w", err)
	}

	fmt.Println(arch)
	return nil
}
