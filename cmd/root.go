package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xcatalysis/catalyst-sdk/go-sdk/baseapp"
	"github.com/0xcatalysis/catalyst-sdk/go-sdk/log"
	"github.com/0xcatalysis/catalyst-sdk/go-sdk/z"
	"github.com/spf13/cobra"

	"github.com/dextr_avs/avs"
)

func NewRootCommand() *cobra.Command {
	// RootCmd represents the base command when called without any subcommands
	var RootCmd = &cobra.Command{
		Use:   "dextr-app",
		Short: "Dextr Token Swap Calculator Application",
		Long:  `An application built with Catalyst SDK that handles token swap operations.`,
	}

	RootCmd.AddCommand(baseapp.GetConfigCommands())
	RootCmd.AddCommand(NewStartCmd())
	RootCmd.AddCommand(NewSwapCmd())

	return RootCmd
}

func NewStartCmd() *cobra.Command {
	var homeDir string

	// StartAppCmd represents the startapp command
	var StartAppCmd = &cobra.Command{
		Use:   "startapp",
		Short: "Start the Dextr Token Swap AVS application",
		Long:  `Start the Dextr Token Swap AVS application with the configured settings.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := log.WithTopic(context.Background(), "startapp")
			// Determine home directory
			catalystHome := homeDir
			if catalystHome == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("error getting home directory: %w", err)
				}
				catalystHome = filepath.Join(home, ".catalyst")
			}

			// Load configuration from JSON file
			configFile := filepath.Join(catalystHome, "config.json")
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				return fmt.Errorf("configuration file not found at %s. Run 'config init' to create one", configFile)
			}

			// Read and parse the config file
			data, err := os.ReadFile(configFile)
			if err != nil {
				return fmt.Errorf("error reading config file: %w", err)
			}

			// Parse into config struct
			var config baseapp.Config
			if err := json.Unmarshal(data, &config); err != nil {
				return fmt.Errorf("error parsing config file: %w", err)
			}

			// Check if any critical configuration is missing
			if config.P2PConfig.TCPListenAddr == "" || config.KeyStoreConfig.KeyDir == "" {
				return fmt.Errorf("missing critical configuration values. Please ensure the config file is properly set up")
			}

			// Create DextrAVS instance with options
			app, err := avs.NewDextrAVS(&config)
			if err != nil {
				return err
			}

			// Log startup information
			log.Info(ctx, "Starting Dextr Token Swap AVS",
				z.Str("listenAddr", config.P2PConfig.TCPListenAddr),
				z.Str("httpPort", config.HTTPPort))

			// Start the app
			if err := app.Start(ctx); err != nil {
				return fmt.Errorf("failed to start app: %w", err)
			}

			log.Info(ctx, "Dextr Token Swap AVS stopped successfully.")
			return nil
		},
	}

	StartAppCmd.Flags().StringVarP(&homeDir, "home", "H", "", "Custom home directory for the node (default: $HOME/.catalyst)")

	return StartAppCmd
}
