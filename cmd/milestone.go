package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"mpvt/internal/api"
	"mpvt/internal/config"
	"mpvt/internal/ui"
)

var (
	msCreateTitle       string
	msCreateType        string
	msCreateDescription string
	msCreateTargetDate  string
	msCreateStatus      string
)

var milestoneCmd = &cobra.Command{
	Use:     "milestone",
	Aliases: []string{"ms"},
	Short:   "Manage project milestones and update roadmaps",
}

var milestoneListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List milestones and roadmaps for the linked project",
	RunE: func(cmd *cobra.Command, args []string) error {
		local, err := activeConfig.EnsureProjectBound()
		if err != nil {
			return err
		}

		milestones, err := apiClient.ListMilestones(cmd.Context(), local.ProjectId)
		if err != nil {
			return fmt.Errorf("failed to fetch milestones: %w", err)
		}

		if flagJSON {
			return outputJSON(milestones)
		}

		if len(milestones) == 0 {
			ui.Info("No milestones or roadmaps defined for this project.")
			fmt.Println("Create one with 'mpvt milestone create'.")
			return nil
		}

		table := ui.NewTable("Active", "ID", "Title", "Type", "Status", "Target Date")

		for _, m := range milestones {
			activeMark := " "
			if m.Id == local.DefaultMilestoneId {
				activeMark = ui.Green("★")
			}

			shortID := ui.ShortID(m.Id, 8)

			targetDate := "-"
			if m.TargetDate != nil && *m.TargetDate != "" {
				targetDate = *m.TargetDate
			}

			table.Append(
				activeMark,
				shortID,
				m.Title,
				m.Type,
				ui.FormatMilestoneBadge(m.Status),
				targetDate,
			)
		}

		table.Render()
		fmt.Printf("\nUse '%s' to switch default milestone for new commits.\n", ui.Bold("mpvt milestone use <id>"))
		return nil
	},
}

var milestoneUseCmd = &cobra.Command{
	Use:   "use [milestone-id-or-none]",
	Short: "Set the default milestone for future progress commits in this workspace",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		local, err := activeConfig.EnsureProjectBound()
		if err != nil {
			return err
		}

		saveDir := ""
		if activeConfig.LocalPath != "" {
			saveDir = filepath.Dir(activeConfig.LocalPath)
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}
			saveDir = cwd
		}

		var targetID string
		if len(args) > 0 {
			targetID = args[0]
		} else {
			// Interactive select
			milestones, err := apiClient.ListMilestones(cmd.Context(), local.ProjectId)
			if err != nil {
				return err
			}
			if len(milestones) == 0 {
				return fmt.Errorf("no milestones available in this project")
			}
			options := []string{"[None] Unset default milestone"}
			for _, m := range milestones {
				options = append(options, fmt.Sprintf("%s (%s, %s)", m.Title, m.Type, m.Status))
			}
			idx, _, err := ui.PromptSelect("Select default milestone", options)
			if err != nil {
				return err
			}
			if idx == 0 {
				targetID = "none"
			} else {
				targetID = milestones[idx-1].Id
			}
		}

		if targetID == "none" || targetID == "null" || targetID == "" {
			local.DefaultMilestoneId = ""
			local.DefaultMilestoneTitle = ""
			if err := config.SaveLocalConfig(saveDir, *local); err != nil {
				return err
			}
			ui.Success("Default milestone unset.")
			return nil
		}

		// Verify existence
		milestones, err := apiClient.ListMilestones(cmd.Context(), local.ProjectId)
		if err != nil {
			return err
		}

		var selectedTitle string
		found := false
		for _, m := range milestones {
			if m.Id == targetID || (len(targetID) >= 8 && strings.HasPrefix(m.Id, targetID)) {
				targetID = m.Id
				selectedTitle = m.Title
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("milestone ID '%s' not found in current project", targetID)
		}

		local.DefaultMilestoneId = targetID
		local.DefaultMilestoneTitle = selectedTitle
		if err := config.SaveLocalConfig(saveDir, *local); err != nil {
			return err
		}

		ui.Successf("Default milestone switched to '%s' (%s)", ui.Bold(selectedTitle), ui.ShortID(targetID, 8))
		return nil
	},
}

var milestoneCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new roadmap or milestone",
	RunE: func(cmd *cobra.Command, args []string) error {
		local, err := activeConfig.EnsureProjectBound()
		if err != nil {
			return err
		}

		title := msCreateTitle
		if title == "" {
			var err error
			title, err = ui.PromptText("Milestone Title", "")
			if err != nil {
				return err
			}
		}
		if title == "" {
			return fmt.Errorf("milestone title cannot be empty")
		}

		mType := msCreateType
		if mType == "" {
			options := []string{
				"ROADMAP (Major phase or version roadmap)",
				"FUTURE_MILESTONE (Future planned goal or target feature)",
			}
			idx, _, err := ui.PromptSelect("Milestone Type", options)
			if err != nil {
				return err
			}
			if idx == 0 {
				mType = "ROADMAP"
			} else {
				mType = "FUTURE_MILESTONE"
			}
		}

		status := msCreateStatus
		if status == "" {
			status = "PLANNED"
		}

		var targetDatePtr *string
		if msCreateTargetDate != "" {
			targetDatePtr = &msCreateTargetDate
		}

		created, err := apiClient.CreateMilestone(cmd.Context(), local.ProjectId, api.CreateMilestoneRequest{
			Title:       title,
			Type:        mType,
			Description: msCreateDescription,
			Status:      status,
			TargetDate:  targetDatePtr,
		})
		if err != nil {
			return fmt.Errorf("failed to create milestone: %w", err)
		}

		if flagJSON {
			return outputJSON(created)
		}

		ui.Successf("Created %s '%s' (%s)", created.Type, ui.Bold(created.Title), ui.ShortID(created.Id, 8))
		return nil
	},
}

var milestoneBindCmd = &cobra.Command{
	Use:   "bind <record-id> [milestone-id-or-null]",
	Short: "Bind or unbind a progress commit to a milestone",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		recordId := args[0]
		targetMilestoneId := ""
		if len(args) > 1 {
			targetMilestoneId = args[1]
		}

		updated, err := apiClient.BindRecord(cmd.Context(), recordId, targetMilestoneId)
		if err != nil {
			return fmt.Errorf("failed to bind record: %w", err)
		}

		if flagJSON {
			return outputJSON(updated)
		}

		if targetMilestoneId == "" || targetMilestoneId == "null" {
			ui.Successf("Record %s unbound from milestone.", ui.ShortID(recordId, 8))
		} else {
			ui.Successf("Record %s bound to milestone %s.", ui.ShortID(recordId, 8), ui.ShortID(targetMilestoneId, 8))
		}
		return nil
	},
}

func init() {
	milestoneCreateCmd.Flags().StringVarP(&msCreateTitle, "title", "t", "", "Milestone title")
	milestoneCreateCmd.Flags().StringVar(&msCreateType, "type", "", "Milestone type: ROADMAP or FUTURE_MILESTONE")
	milestoneCreateCmd.Flags().StringVarP(&msCreateDescription, "description", "d", "", "Milestone description")
	milestoneCreateCmd.Flags().StringVar(&msCreateTargetDate, "target-date", "", "Target date (YYYY-MM-DD)")
	milestoneCreateCmd.Flags().StringVar(&msCreateStatus, "status", "PLANNED", "Status (PLANNED/IN_PROGRESS/COMPLETED)")

	milestoneCmd.AddCommand(milestoneListCmd)
	milestoneCmd.AddCommand(milestoneUseCmd)
	milestoneCmd.AddCommand(milestoneCreateCmd)
	milestoneCmd.AddCommand(milestoneBindCmd)

	rootCmd.AddCommand(milestoneCmd)
}
