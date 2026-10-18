package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"mpvt/internal/api"
	"mpvt/internal/git"
	"mpvt/internal/ui"
)

var (
	commitTitle       string
	commitDescription string
	commitMilestone   string
	commitTime        string
	commitFromGit     bool
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Record and submit a non-code progress commit to ProjectVitals",
	Long: `Record what you accomplished without leaking source code.
Solves accountability and enables transparent progress tracking for developers.
Supports automatic extraction from your local git commits via --from-git.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		local, err := activeConfig.EnsureProjectBound()
		if err != nil {
			return err
		}

		// 1. Resolve Git Commit if --from-git is specified
		if commitFromGit {
			if !git.IsGitRepo(cmd.Context()) {
				return fmt.Errorf("current directory is not a git repository")
			}
			lastCommit, err := git.GetLastCommit(cmd.Context())
			if err != nil {
				return fmt.Errorf("git repository has no commits yet or git log failed: %w", err)
			}
			if lastCommit == nil {
				return fmt.Errorf("git repository has no commits yet. Please create an initial commit first or specify -m")
			}
			if commitTitle == "" {
				commitTitle = lastCommit.Subject
			}
			if commitDescription == "" && lastCommit.Body != "" {
				commitDescription = lastCommit.Body
			}
			ui.Infof("Extracted from git commit [%s]: %s", ui.ShortID(lastCommit.Hash, 8), commitTitle)
		}

		// 2. Interactive prompt if no message provided
		if commitTitle == "" {
			// Check if we can suggest from git
			if git.IsGitRepo(cmd.Context()) {
				if lastCommit, err := git.GetLastCommit(cmd.Context()); err == nil && lastCommit != nil && lastCommit.Subject != "" {
					fmt.Printf("Detected recent git commit: %s %s\n", ui.Yellow(ui.ShortID(lastCommit.Hash, 7)), ui.Bold(lastCommit.Subject))
					if ui.PromptConfirm("Use this git commit message?", true) {
						commitTitle = lastCommit.Subject
						if lastCommit.Body != "" {
							commitDescription = lastCommit.Body
						}
					}
				}
			}

			if commitTitle == "" {
				commitTitle, err = ui.PromptText("Progress Title (What did you do?)", "")
				if err != nil {
					return err
				}
			}

			if commitTitle == "" {
				return fmt.Errorf("commit title is required")
			}

			if commitDescription == "" {
				commitDescription, err = ui.PromptMultiline("Detailed Description (Optional)")
				if err != nil {
					return err
				}
			}
		}

		// 3. Resolve Milestone
		targetMilestoneId := commitMilestone
		if targetMilestoneId == "" {
			targetMilestoneId = local.DefaultMilestoneId
		}

		// Interactive milestone selector if not provided and in interactive mode
		if targetMilestoneId == "" && !commitFromGit && commitTitle != "" && len(args) == 0 && !cmd.Flags().Changed("message") {
			milestones, err := apiClient.ListMilestones(cmd.Context(), local.ProjectId)
			if err == nil && len(milestones) > 0 {
				options := []string{"[None] Do not bind to milestone"}
				for _, m := range milestones {
					options = append(options, fmt.Sprintf("%s (%s, %s)", m.Title, m.Type, m.Status))
				}
				idx, _, err := ui.PromptSelect("Associate with milestone?", options)
				if err == nil && idx > 0 {
					targetMilestoneId = milestones[idx-1].Id
				}
			}
		}

		// 4. Resolve committed time
		var commitTimestamp *time.Time
		if commitTime != "" {
			parsed, err := time.Parse(time.RFC3339, commitTime)
			if err != nil {
				parsed, err = time.Parse("2006-01-02 15:04:05", commitTime)
			}
			if err != nil {
				return fmt.Errorf("invalid time format (use RFC3339 or 'YYYY-MM-DD HH:MM:SS'): %w", err)
			}
			commitTimestamp = &parsed
		}

		ui.Infof("Submitting progress record to project '%s' ...", local.ProjectSlug)
		record, err := apiClient.CreateRecord(cmd.Context(), local.ProjectId, api.CreateRecordRequest{
			Title:       commitTitle,
			Description: commitDescription,
			MilestoneID: targetMilestoneId,
			CommittedAt: commitTimestamp,
		})
		if err != nil {
			return fmt.Errorf("failed to submit record: %w", err)
		}

		if flagJSON {
			return outputJSON(record)
		}

		fmt.Println()
		ui.Success("Progress commit recorded successfully!")
		fmt.Printf("● Record ID:    %s\n", ui.Yellow(record.Id))
		fmt.Printf("● Title:        %s\n", ui.Bold(record.Title))
		if record.Description != nil && *record.Description != "" {
			fmt.Printf("● Description:  %s\n", strings.ReplaceAll(*record.Description, "\n", "\n                "))
		}
		if record.MilestoneId != nil && *record.MilestoneId != "" {
			fmt.Printf("● Milestone ID: %s\n", ui.Cyan(*record.MilestoneId))
		}
		fmt.Printf("● Committed At: %s\n", record.CommittedAt.Format("2006-01-02 15:04:05"))

		return nil
	},
}

func init() {
	commitCmd.Flags().StringVarP(&commitTitle, "message", "m", "", "Commit title (What was done)")
	commitCmd.Flags().StringVarP(&commitDescription, "description", "d", "", "Detailed description/impact")
	commitCmd.Flags().StringVar(&commitMilestone, "milestone", "", "Milestone UUID to bind")
	commitCmd.Flags().StringVar(&commitTime, "time", "", "Custom commit time (RFC3339 or 'YYYY-MM-DD HH:MM:SS')")
	commitCmd.Flags().BoolVarP(&commitFromGit, "from-git", "g", false, "Auto-extract title and description from recent local git commit")

	rootCmd.AddCommand(commitCmd)
}
