// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"mpvt/internal/api"
	"mpvt/internal/config"
	"mpvt/internal/git"
	"mpvt/internal/ui"
)

var (
	projectListStatus     string
	projectListQuery      string
	projectListPage       int
	projectListLimit      int
	projectCreateSlug     string
	projectCreateDesc     string
	projectCreateVis      string
	projectCreateInitStat string
)

var projectCmd = &cobra.Command{
	Use:     "project",
	Aliases: []string{"proj"},
	Short:   "Manage and browse ProjectVitals projects",
}

var projectListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List projects on the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := apiClient.ListProjectsPaged(cmd.Context(), projectListPage, projectListLimit, projectListStatus, projectListQuery)
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		if flagJSON {
			return outputJSON(resp)
		}

		if len(resp.Data) == 0 {
			ui.Info("No projects found.")
			return nil
		}

		table := ui.NewTable("Slug", "Title", "Status", "Visibility", "Members", "Owner")

		for _, p := range resp.Data {
			ownerEmail := "-"
			if p.Owner != nil {
				ownerEmail = p.Owner.DisplayName()
			}

			table.Append(
				p.Slug,
				p.Title,
				ui.FormatStatusBadge(p.Status),
				p.Visibility,
				fmt.Sprintf("%d", p.MemberCount),
				ownerEmail,
			)
		}

		table.Render()

		if resp.Meta != nil && resp.Meta.TotalPages > 1 {
			fmt.Printf("\nPage %d of %d (Total %d projects)\n", resp.Meta.Page, resp.Meta.TotalPages, resp.Meta.Total)
		}

		return nil
	},
}

var projectViewCmd = &cobra.Command{
	Use:   "view [id-or-slug]",
	Short: "View detailed information about a project",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetID := ""
		if len(args) > 0 {
			targetID = args[0]
		} else if activeConfig.Local != nil {
			targetID = activeConfig.Local.ProjectId
		} else {
			return fmt.Errorf("specify a project slug or run inside a linked project directory")
		}

		project, err := apiClient.GetProject(cmd.Context(), targetID)
		if err != nil {
			return fmt.Errorf("failed to fetch project '%s': %w", targetID, err)
		}

		if flagJSON {
			return outputJSON(project)
		}

		fmt.Printf("● Title:       %s\n", ui.Bold(project.Title))
		fmt.Printf("● Slug:        %s\n", ui.Cyan(project.Slug))
		fmt.Printf("● UUID:        %s\n", project.Id)
		fmt.Printf("● Status:      %s\n", ui.FormatStatusBadge(project.Status))
		fmt.Printf("● Visibility:  %s\n", project.Visibility)
		if project.Description != nil && *project.Description != "" {
			fmt.Printf("● Description: %s\n", *project.Description)
		}
		if project.Owner != nil {
			fmt.Printf("● Owner:       %s (%s)\n", project.Owner.DisplayName(), project.Owner.Role)
		}

		members, err := apiClient.ListMembers(cmd.Context(), project.Id)
		if err == nil && len(members) > 0 {
			fmt.Printf("\nMembers (%d):\n", len(members))
			for _, m := range members {
				name := "Unknown"
				if m.User != nil {
					name = m.User.DisplayName()
				}
				fmt.Printf("  • %s - %s\n", name, ui.Cyan(m.Role))
			}
		}

		return nil
	},
}

var projectCreateCmd = &cobra.Command{
	Use:     "create [title]",
	Aliases: []string{"new"},
	Short:   "Create a new project on ProjectVitals",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := ""
		if len(args) > 0 {
			title = args[0]
		} else {
			var err error
			title, err = ui.PromptText("Project Title", "")
			if err != nil {
				return err
			}
		}
		if title == "" {
			return fmt.Errorf("project title cannot be empty")
		}

		slug := projectCreateSlug
		if slug == "" {
			suggestedSlug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(title), " ", "-"))
			var err error
			slug, err = ui.PromptText("Project Slug (URL friendly, e.g. my-app)", suggestedSlug)
			if err != nil {
				return err
			}
		}
		if slug == "" {
			return fmt.Errorf("slug cannot be empty")
		}

		desc := projectCreateDesc
		if desc == "" && len(args) == 0 && !cmd.Flags().Changed("description") {
			desc, _ = ui.PromptText("Project Description (Optional)", "")
		}

		vis := projectCreateVis
		if vis == "" {
			vis = "PUBLIC"
		}

		stat := projectCreateInitStat
		if stat == "" {
			stat = "IN_DEVELOPMENT"
		}

		project, err := apiClient.CreateProject(cmd.Context(), api.CreateProjectRequest{
			Title:       title,
			Slug:        slug,
			Description: desc,
			Visibility:  vis,
			Status:      stat,
		})
		if err != nil {
			return fmt.Errorf("failed to create project: %w", err)
		}

		if flagJSON {
			return outputJSON(project)
		}

		ui.Successf("Project '%s' created successfully!", ui.Bold(project.Title))
		fmt.Printf("● Slug:       %s\n", ui.Cyan(project.Slug))
		fmt.Printf("● UUID:       %s\n", project.Id)
		fmt.Printf("● Visibility: %s\n", project.Visibility)
		fmt.Printf("● Status:     %s\n", ui.FormatStatusBadge(project.Status))
		fmt.Println()

		// Prompt to link current directory if not already bound
		if activeConfig.Local == nil || activeConfig.Local.ProjectId == "" {
			if ui.PromptConfirm("Bind current directory to this new project now?", true) {
				saveDir, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get current working directory: %w", err)
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
					return fmt.Errorf("failed to save local workspace config: %w", err)
				}
				ui.Successf("Bound workspace to %s (%s)", ui.Bold(project.Title), ui.Cyan(project.Slug))
				if git.IsGitRepo(cmd.Context()) {
					_, _ = git.EnsureGitIgnore(saveDir)
				}
			} else {
				ui.Infof("Run '%s' anytime to link your workspace.", ui.Bold("mpvt link "+project.Slug))
			}
		} else {
			ui.Infof("Run '%s' to link your workspace to this new project.", ui.Bold("mpvt link "+project.Slug))
		}
		return nil
	},
}

var projectStateCmd = &cobra.Command{
	Use:   "set-status <IN_DEVELOPMENT|PUBLISHED|PLANNING|ARCHIVED> [id-or-slug]",
	Short: "Update project lifecycle status (Admin/Owner only)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		newStatus := args[0]
		targetID := ""
		if len(args) > 1 {
			targetID = args[1]
		} else if activeConfig.Local != nil {
			targetID = activeConfig.Local.ProjectId
		} else {
			return fmt.Errorf("specify project slug or run inside a linked project directory")
		}

		if targetID != "" {
			p, err := apiClient.GetProject(cmd.Context(), targetID)
			if err == nil && p != nil && p.Id != "" {
				targetID = p.Id
			}
		}

		updated, err := apiClient.UpdateProjectStatus(cmd.Context(), targetID, newStatus)
		if err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}

		if flagJSON {
			return outputJSON(updated)
		}

		ui.Successf("Project status updated to %s", ui.FormatStatusBadge(updated.Status))
		return nil
	},
}

func init() {
	projectListCmd.Flags().StringVar(&projectListStatus, "status", "", "Filter by status: IN_DEVELOPMENT, PUBLISHED, PLANNING, ARCHIVED")
	projectListCmd.Flags().StringVarP(&projectListQuery, "query", "q", "", "Search keywords")
	projectListCmd.Flags().IntVar(&projectListPage, "page", 1, "Page number")
	projectListCmd.Flags().IntVar(&projectListLimit, "limit", 20, "Page limit")

	projectCreateCmd.Flags().StringVarP(&projectCreateSlug, "slug", "s", "", "Project unique slug")
	projectCreateCmd.Flags().StringVarP(&projectCreateDesc, "description", "d", "", "Project description")
	projectCreateCmd.Flags().StringVar(&projectCreateVis, "visibility", "PUBLIC", "Visibility: PUBLIC or PRIVATE")
	projectCreateCmd.Flags().StringVar(&projectCreateInitStat, "status", "IN_DEVELOPMENT", "Initial status: IN_DEVELOPMENT, PUBLISHED, PLANNING")

	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectViewCmd)
	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectStateCmd)

	rootCmd.AddCommand(projectCmd)

	// Top-level 'mpvt new' / 'mpvt create' shortcut
	newCmd := &cobra.Command{
		Use:     "new [title]",
		Aliases: []string{"create"},
		Short:   "Create a new project on ProjectVitals (Shortcut for 'mpvt project create')",
		Args:    cobra.MaximumNArgs(1),
		RunE:    projectCreateCmd.RunE,
	}
	newCmd.Flags().StringVarP(&projectCreateSlug, "slug", "s", "", "Project unique slug")
	newCmd.Flags().StringVarP(&projectCreateDesc, "description", "d", "", "Project description")
	newCmd.Flags().StringVar(&projectCreateVis, "visibility", "PUBLIC", "Visibility: PUBLIC or PRIVATE")
	newCmd.Flags().StringVar(&projectCreateInitStat, "status", "IN_DEVELOPMENT", "Initial status: IN_DEVELOPMENT, PUBLISHED, PLANNING")
	rootCmd.AddCommand(newCmd)
}
