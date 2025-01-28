package cmd

import (
	"context"
	"os"

	"github.com/loft-sh/devpod-provider-kubernetes/pkg/kubernetes"
	"github.com/loft-sh/devpod-provider-kubernetes/pkg/options"
	"github.com/loft-sh/log"
	"github.com/spf13/cobra"
)

// RenderTemplateCmd holds the cmd flags
type RenderTemplateCmd struct{}

// NewRenderTemplateCmd defines a command
func NewRenderTemplateCmd() *cobra.Command {
	cmd := &RenderTemplateCmd{}
	templateCmd := &cobra.Command{
		Use:   "render-template",
		Short: "Render Template for provider",
		RunE: func(_ *cobra.Command, args []string) error {
			options, err := options.FromEnv()
			if err != nil {
				return err
			}

			return cmd.Run(context.Background(), options, log.Default.ErrorStreamOnly())
		},
	}

	return templateCmd
}

// Run runs the command logic
func (cmd *RenderTemplateCmd) Run(ctx context.Context, options *options.Options, log log.Logger) error {
	verbose := os.Getenv("DEVPOD_VERBOSE") == "true"
	return kubernetes.NewKubernetesDriver(options, log).RenderTemplate(ctx, options.DevContainerID, verbose)
}
