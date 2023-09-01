package cmd

import (
	"os"
	"os/exec"

	log2 "github.com/loft-sh/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

// NewRootCmd returns a new root command
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "devpod-provider-kubernetes",
		Short:         "Kubernetes Provider commands",
		SilenceErrors: true,
		SilenceUsage:  true,

		PersistentPreRunE: func(cobraCmd *cobra.Command, args []string) error {
			if os.Getenv("DEVPOD_DEBUG") == "true" {
				log2.Default.SetLevel(logrus.DebugLevel)
			}

			log2.Default.MakeRaw()
			return nil
		},
	}

	return cmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// build the root command
	rootCmd := BuildRoot()

	// execute command
	err := rootCmd.Execute()
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			os.Exit(exitErr.ExitStatus())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if len(exitErr.Stderr) > 0 {
				log2.Default.ErrorStreamOnly().Error(string(exitErr.Stderr))
			}
			os.Exit(exitErr.ExitCode())
		}

		log2.Default.Fatal(err)
	}
}

// BuildRoot creates a new root command from the
func BuildRoot() *cobra.Command {
	rootCmd := NewRootCmd()
	rootCmd.AddCommand(NewStartCmd())
	rootCmd.AddCommand(NewStopCmd())
	rootCmd.AddCommand(NewDeleteCmd())
	rootCmd.AddCommand(NewRunCmd())
	rootCmd.AddCommand(NewFindCmd())
	rootCmd.AddCommand(NewCommandCmd())
	rootCmd.AddCommand(NewTargetArchitectureCmd())
	return rootCmd
}
