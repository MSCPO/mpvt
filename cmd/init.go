package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"mpvt/internal/config"
	"mpvt/internal/git"
	"mpvt/internal/ui"
)

var (
	flagNoGitIgnore bool
)

var initCmd = &cobra.Command{
	Use:     "init [project-slug-or-id]",
	Aliases: []string{"link"},
	Short:   "Bind the current directory to a ProjectVitals project",
	Long: `Initialize or link a ProjectVitals project to the current workspace.
Creates a local .mpvt/config.json file to track the project ID and active milestone.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var targetSlug string
		if len(args) > 0 {
			targetSlug = args[0]
		} else {
			// List projects for interactive selection or prompt
			ui.Infof("Fetching available projects from %s ...", activeConfig.GetEffectiveEndpoint())
			resp, err := apiClient.ListProjectsPaged(cmd.Context(), 1, 10, "", "")
			if err != nil {
				ui.Warningf("Could not list projects automatically: %v", err)
			} else if resp != nil && len(resp.Data) > 0 {
				options := make([]string, len(resp.Data))
				for i, p := range resp.Data {
					options[i] = fmt.Sprintf("%s (%s) - %s", p.Title, p.Slug, p.Status)
				}
				idx, _, err := ui.PromptSelect("Select project to link", options)
				if err == nil && idx >= 0 {
					targetSlug = resp.Data[idx].Slug
				}
			}

			if targetSlug == "" {
				var err error
				targetSlug, err = ui.PromptText("Enter Project Slug or UUID", "")
				if err != nil {
					return err
				}
			}
		}

		if targetSlug == "" {
			return fmt.Errorf("project slug or UUID cannot be empty")
		}

		ui.Infof("Verifying project '%s' ...", targetSlug)
		project, err := apiClient.GetProject(cmd.Context(), targetSlug)
		if err != nil {
			return fmt.Errorf("failed to retrieve project '%s': %w", targetSlug, err)
		}

		saveDir, err := os.Getwd()
		if err != nil {
			return err
		}
		if repoRoot, err := git.GetRepoRoot(cmd.Context()); err == nil && repoRoot != "" {
			saveDir = repoRoot
		}

		localCfg := config.LocalConfig{
			ProjectId:    project.Id,
			ProjectSlug:  project.Slug,
			ProjectTitle: project.Title,
			Endpoint:     activeConfig.GetEffectiveEndpoint(),
		}

		if err := config.SaveLocalConfig(saveDir, localCfg); err != nil {
			return fmt.Errorf("failed to save local configuration: %w", err)
		}

		ui.Successf("Workspace bound to project %s (%s)", ui.Bold(project.Title), ui.Cyan(project.Slug))
		fmt.Printf("● Status:     %s\n", ui.FormatStatusBadge(project.Status))
		if project.Owner != nil {
			fmt.Printf("● Owner:      %s (%s)\n", project.Owner.DisplayName(), project.Owner.Role)
		}
		fmt.Printf("● Local Dir:  %s\n", filepath.Join(saveDir, config.LocalDirName))

		// Check git integration
		if git.IsGitRepo(cmd.Context()) && !flagNoGitIgnore {
			added, err := git.EnsureGitIgnore(saveDir)
			if err == nil && added {
				ui.Success("Added .mpvt/ to .gitignore")
			}
		}

		fmt.Println()
		ui.Infof("You can now run '%s' to record changes or '%s' to inspect project status.", ui.Bold("mpvt commit"), ui.Bold("mpvt status"))
		return nil
	},
}

var unlinkCmd = &cobra.Command{
	Use:   "unlink",
	Short: "Unbind the current directory from ProjectVitals",
	RunE: func(cmd *cobra.Command, args []string) error {
		local, localPath, err := config.FindLocalConfig()
		if err != nil {
			return fmt.Errorf("failed to check local config: %w", err)
		}
		if local == nil {
			return fmt.Errorf("no linked project in current workspace")
		}

		if err := os.RemoveAll(localPath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", localPath, err)
		}

		ui.Successf("Unlinked from project '%s' (%s)", local.ProjectTitle, local.ProjectSlug)
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&flagNoGitIgnore, "no-gitignore", false, "Do not add .mpvt/ to .gitignore")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(unlinkCmd)
}
