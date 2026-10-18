package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"mpvt/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show project status, lifecycle state, active milestone, and recent commits",
	RunE: func(cmd *cobra.Command, args []string) error {
		local, err := activeConfig.EnsureProjectBound()
		if err != nil {
			return err
		}

		project, err := apiClient.GetProject(cmd.Context(), local.ProjectId)
		if err != nil {
			return fmt.Errorf("failed to fetch project details: %w", err)
		}

		if flagJSON {
			return outputJSON(project)
		}

		fmt.Printf("● Project:    %s (%s)\n", ui.Bold(project.Title), ui.Cyan(project.Slug))
		fmt.Printf("● Status:     %s\n", ui.FormatStatusBadge(project.Status))
		fmt.Printf("● Visibility: %s\n", ui.Dim(project.Visibility))
		if project.Description != nil && *project.Description != "" {
			fmt.Printf("● About:      %s\n", *project.Description)
		}

		if project.Owner != nil {
			fmt.Printf("● Owner:      %s\n", project.Owner.DisplayName())
		}

		// Milestone info
		if local.DefaultMilestoneId != "" {
			milestoneTitle := local.DefaultMilestoneTitle
			if milestoneTitle == "" {
				milestoneTitle = local.DefaultMilestoneId
			}
			fmt.Printf("● Active MS:  %s (%s)\n", ui.Green(milestoneTitle), local.DefaultMilestoneId)
		} else {
			fmt.Printf("● Active MS:  %s (Run 'mpvt milestone use <id>' to set)\n", ui.Dim("None"))
		}

		// Show lifecycle contextual guidance
		fmt.Println()
		switch strings.ToUpper(project.Status) {
		case "IN_DEVELOPMENT":
			ui.Infof("Phase: [Development] Ready to submit progress records via '%s'.", ui.Bold("mpvt commit"))
		case "PLANNING":
			ui.Infof("Phase: [Planning] Concept proposals enabled! Submit PRs via '%s'.", ui.Bold("mpvt pr create"))
		case "PUBLISHED":
			ui.Infof("Phase: [Published] Release maintenance. Plan new milestones via '%s'.", ui.Bold("mpvt milestone create"))
		case "ARCHIVED":
			ui.Warning("Phase: [Archived] Project is currently read-only locked.")
		}

		// Fetch and display recent records
		recordsResp, err := apiClient.ListRecordsWithFilter(cmd.Context(), project.Id, "", "", 1, 3)
		if err == nil && recordsResp != nil && len(recordsResp.Data) > 0 {
			fmt.Println()
			fmt.Println(ui.Bold("Recent Progress Commits:"))
			for _, r := range recordsResp.Data {
				shortID := ui.ShortID(r.Id, 8)
				authorName := "Anonymous"
				if r.Author != nil {
					authorName = r.Author.DisplayName()
				}
				msLabel := ""
				if r.Milestone != nil {
					msLabel = fmt.Sprintf(" [%s]", r.Milestone.Title)
				}
				fmt.Printf("  %s %s - %s %s%s\n",
					ui.Yellow(shortID),
					ui.Dim(r.CommittedAt.Format("01-02 15:04")),
					r.Title,
					ui.Dim("by "+authorName),
					ui.Cyan(msLabel),
				)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
