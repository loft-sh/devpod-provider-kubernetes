package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/loft-sh/devpod-provider-kubernetes/pkg/kubernetes"
	"github.com/loft-sh/devpod-provider-kubernetes/pkg/options"
	"github.com/loft-sh/log"
	"github.com/spf13/cobra"
)

// FindCmd holds the cmd flags
type FindCmd struct{}

// NewFindCmd defines a command
func NewFindCmd() *cobra.Command {
	cmd := &FindCmd{}
	findCmd := &cobra.Command{
		Use:   "find",
		Short: "Find a container",
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := options.FromEnv()
			if err != nil {
				return err
			}

			return cmd.Run(context.Background(), options, log.Default.ErrorStreamOnly())
		},
	}

	return findCmd
}

// Run runs the command logic
func (cmd *FindCmd) Run(ctx context.Context, options *options.Options, log log.Logger) error {
	containerDetails, err := kubernetes.NewKubernetesDriver(options, log).FindDevContainer(ctx, options.DevContainerID)
	if err != nil {
		return err
	} else if containerDetails == nil {
		return nil
	}

	out, err := json.Marshal(containerDetails)
	if err != nil {
		return fmt.Errorf("error marshalling container details: %w", err)
	}

	fmt.Println(string(out))
	return nil
}
