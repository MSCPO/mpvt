// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"mpvt/internal/api"
	"mpvt/internal/config"
	"mpvt/internal/ui"
)

var (
	flagEndpoint string
	flagToken    string
	flagProject  string
	flagJSON     bool

	activeConfig *config.Config
	apiClient    *api.Client
)

var rootCmd = &cobra.Command{
	Use:   "mpvt",
	Short: "mpvt - MSCPO ProjectVitals CLI (Developer progress & vitals tracking tool)",
	Long: `mpvt (MSCPO ProjectVitals Tool) is a Git-like developer CLI for the ProjectVitals platform.
It allows developers to record and submit progress commits (without source code leak),
track project milestones, manage concept proposals, and inspect vitals history.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadActiveConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}
		activeConfig = cfg

		// Command line flag overrides
		if flagEndpoint != "" {
			activeConfig.Global.Endpoint = flagEndpoint
		}
		if flagToken != "" {
			activeConfig.Global.Token = flagToken
		}

		apiClient = api.NewClient(activeConfig)

		cmdName := cmd.Name()
		isAuthCmd := cmdName == "whoami" || (cmd.Parent() != nil && cmd.Parent().Name() == "auth")
		isOfflineCmd := isAuthCmd || cmdName == "version" || cmdName == "help" || cmdName == "completion" || cmdName == "logout" || cmdName == "login" || cmdName == "init" || cmdName == "link" || cmdName == "unlink"
		if flagProject != "" && !isOfflineCmd {
			if activeConfig.Local == nil {
				activeConfig.Local = &config.LocalConfig{}
			}
			resolveCtx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
			defer cancel()
			proj, err := apiClient.GetProject(resolveCtx, flagProject)
			if err != nil {
				return fmt.Errorf("failed to resolve project %q: %w", flagProject, err)
			}
			if proj == nil {
				return fmt.Errorf("project %q not found", flagProject)
			}
			activeConfig.Local.ProjectId = proj.Id
			activeConfig.Local.ProjectSlug = proj.Slug
			activeConfig.Local.ProjectTitle = proj.Title
		}

		return nil
	},
}

// SetApiClient configures or overrides the active API client (useful for dependency injection and tests)
func SetApiClient(c *api.Client) {
	apiClient = c
}

// GetApiClient returns the current active API client
func GetApiClient() *api.Client {
	return apiClient
}

// SetActiveConfig sets or overrides the active configuration
func SetActiveConfig(cfg *config.Config) {
	activeConfig = cfg
}

// GetActiveConfig returns the current active configuration
func GetActiveConfig() *config.Config {
	return activeConfig
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "View current authentication status and server connection (Shortcut for 'mpvt auth status')",
	RunE:  authStatusCmd.RunE,
}

func ExecuteContext(ctx context.Context) {
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		ui.Errorf("%v", err)
		os.Exit(1)
	}
}

func Execute() {
	ExecuteContext(context.Background())
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagEndpoint, "endpoint", "e", "", "ProjectVitals API endpoint URL (default: https://api.mscpo.com/api/projectvitals)")
	rootCmd.PersistentFlags().StringVarP(&flagToken, "token", "t", "", "Authentication Bearer token")
	rootCmd.PersistentFlags().StringVarP(&flagProject, "project", "p", "", "Target project UUID or slug override")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output response in JSON format")

	rootCmd.AddCommand(whoamiCmd)
}

// outputJSON outputs an object as indented JSON
func outputJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
