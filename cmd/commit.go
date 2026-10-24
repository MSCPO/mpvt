package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"mpvt/internal/api"
	"mpvt/internal/config"
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

func resetCommitFlags() {
	commitTitle = ""
	commitDescription = ""
	commitMilestone = ""
	commitTime = ""
	commitFromGit = false
}

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Record and submit a non-code progress commit to ProjectVitals",
	Long: `Record what you accomplished without leaking source code.
Solves accountability and enables transparent progress tracking for developers.
Supports automatic extraction from your local git commits via --from-git.`,
	PostRun: func(cmd *cobra.Command, args []string) {
		resetCommitFlags()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		local, err := activeConfig.EnsureProjectBound()
		if err != nil {
			return err
		}

		// 1. Resolve Git Commit if --from-git is specified
		if commitFromGit {
			if err := resolveFromGit(cmd); err != nil {
				return err
			}
		}

		// 2. Interactive prompt if title still empty
		if commitTitle == "" {
			if err := promptCommitDetails(cmd); err != nil {
				return err
			}
		}

		// 3. Resolve Milestone
		targetMilestoneId := resolveMilestone(cmd, local, len(args) == 0)

		// 4. Resolve committed time
		commitTimestamp, err := parseCommitTimestamp(commitTime)
		if err != nil {
			return err
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

		printCommitSuccess(record)
		return nil
	},
}

func resolveFromGit(cmd *cobra.Command) error {
	lastCommit, err := git.GetLastCommit(cmd.Context())
	if err != nil {
		if !git.IsGitRepo(cmd.Context()) {
			return fmt.Errorf("current directory is not a git repository")
		}
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
	return nil
}

func promptCommitDetails(cmd *cobra.Command) error {
	// Check if we can suggest from git
	if lastCommit, err := git.GetLastCommit(cmd.Context()); err == nil && lastCommit != nil && lastCommit.Subject != "" {
		fmt.Printf("Detected recent git commit: %s %s\n", ui.Yellow(ui.ShortID(lastCommit.Hash, 7)), ui.Bold(lastCommit.Subject))
		if ui.PromptConfirm("Use this git commit message?", true) {
			commitTitle = lastCommit.Subject
			if lastCommit.Body != "" {
				commitDescription = lastCommit.Body
			}
		}
	}

	if commitTitle == "" {
		var err error
		commitTitle, err = ui.PromptText("Progress Title (What did you do?)", "")
		if err != nil {
			return err
		}
	}

	if commitTitle == "" {
		return fmt.Errorf("commit title is required")
	}

	if commitDescription == "" {
		var err error
		commitDescription, err = ui.PromptMultiline("Detailed Description (Optional)")
		if err != nil {
			return err
		}
	}
	return nil
}

func resolveMilestone(cmd *cobra.Command, local *config.LocalConfig, isInteractiveCandidate bool) string {
	targetMilestoneId := commitMilestone
	if targetMilestoneId == "" && !cmd.Flags().Changed("milestone") {
		targetMilestoneId = local.DefaultMilestoneId
	}
	if strings.EqualFold(targetMilestoneId, "none") || strings.EqualFold(targetMilestoneId, "null") {
		targetMilestoneId = ""
	}

	// Interactive milestone selector if not provided, not explicitly set via flag, and in interactive mode
	if targetMilestoneId == "" && !cmd.Flags().Changed("milestone") && !commitFromGit && commitTitle != "" && isInteractiveCandidate && !cmd.Flags().Changed("message") {
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
	return targetMilestoneId
}

func parseCommitTimestamp(timeStr string) (*time.Time, error) {
	if timeStr == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		parsed, err = time.ParseInLocation("2006-01-02 15:04:05", timeStr, time.Local)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid time format (use RFC3339 or 'YYYY-MM-DD HH:MM:SS'): %w", err)
	}
	return &parsed, nil
}

func printCommitSuccess(record *api.Record) {
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
}

func init() {
	commitCmd.Flags().StringVarP(&commitTitle, "message", "m", "", "Commit title (What was done)")
	commitCmd.Flags().StringVarP(&commitDescription, "description", "d", "", "Detailed description/impact")
	commitCmd.Flags().StringVar(&commitMilestone, "milestone", "", "Milestone UUID to bind")
	commitCmd.Flags().StringVar(&commitTime, "time", "", "Custom commit time (RFC3339 or 'YYYY-MM-DD HH:MM:SS')")
	commitCmd.Flags().BoolVarP(&commitFromGit, "from-git", "g", false, "Auto-extract title and description from recent local git commit")

	rootCmd.AddCommand(commitCmd)
}
