// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"mpvt/internal/api"
	"mpvt/internal/git"
	"mpvt/internal/ui"
)

var (
	importRev           string
	importSince         string
	importUntil         string
	importAuthor        string
	importLimit         int
	importMilestone     string
	importSkipExisting  = true
	importIncludeHash   bool
	importIncludeAuthor bool
	importDryRun        bool
	importYes           bool
	importReverse       = true
	importNoMerges      bool
)

func resetImportFlags() {
	importRev = ""
	importSince = ""
	importUntil = ""
	importAuthor = ""
	importLimit = 0
	importMilestone = ""
	importSkipExisting = true
	importIncludeHash = false
	importIncludeAuthor = false
	importDryRun = false
	importYes = false
	importReverse = true
	importNoMerges = false
}

type ImportResult struct {
	Total    int           `json:"total"`
	Imported int           `json:"imported"`
	Skipped  int           `json:"skipped"`
	Failed   int           `json:"failed"`
	Records  []*api.Record `json:"records,omitempty"`
}

var importCmd = &cobra.Command{
	Use:     "import [git]",
	Aliases: []string{"import-git", "git-import"},
	Short:   "Bulk import Git commits into ProjectVitals",
	Long: `Bulk import historical commits from your local Git repository into ProjectVitals as progress records.
Supports revision ranges, date filtering (--since, --until), author filtering, milestone binding,
smart deduplication (--skip-existing), and dry-run preview (--dry-run).`,
	PostRun: func(cmd *cobra.Command, args []string) {
		resetImportFlags()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGitImport(cmd, args)
	},
}

var importGitCmd = &cobra.Command{
	Use:     "git",
	Aliases: []string{"commits"},
	Short:   "Bulk import Git commits into ProjectVitals",
	Long:    importCmd.Long,
	PostRun: func(cmd *cobra.Command, args []string) {
		resetImportFlags()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGitImport(cmd, args)
	},
}

func runGitImport(cmd *cobra.Command, args []string) error {
	local, err := activeConfig.EnsureProjectBound()
	if err != nil {
		return err
	}

	if !git.IsGitRepo(cmd.Context()) {
		return fmt.Errorf("current directory is not a git repository")
	}

	// 1. Resolve target milestone
	targetMilestoneId := importMilestone
	if targetMilestoneId == "" && !cmd.Flags().Changed("milestone") {
		targetMilestoneId = local.DefaultMilestoneId
	}
	if strings.EqualFold(targetMilestoneId, "none") || strings.EqualFold(targetMilestoneId, "null") {
		targetMilestoneId = ""
	}

	// 2. Fetch git commits
	filter := git.CommitFilter{
		RevisionRange: importRev,
		Since:         importSince,
		Until:         importUntil,
		Author:        importAuthor,
		Limit:         importLimit,
		Reverse:       importReverse,
		NoMerges:      importNoMerges,
	}

	commits, err := git.ListCommits(cmd.Context(), filter)
	if err != nil {
		return fmt.Errorf("failed to retrieve git commits: %w", err)
	}

	if len(commits) == 0 {
		ui.Info("No git commits found matching the specified criteria.")
		if flagJSON {
			return outputJSON(ImportResult{Total: 0})
		}
		return nil
	}

	// 3. Deduplication check against existing records in project
	var existingFingerprints map[string]bool
	if importSkipExisting {
		existingFingerprints = fetchExistingRecordFingerprints(cmd.Context(), local.ProjectId)
	}

	var toImport []*git.CommitInfo
	var skippedCount int
	for _, c := range commits {
		isDup := false
		if existingFingerprints != nil {
			if existingFingerprints["hash:"+c.Hash] {
				isDup = true
			}
			if !isDup && c.CommittedAt != nil {
				keyTime := c.Subject + "|" + c.CommittedAt.Format(time.RFC3339)
				keySec := fmt.Sprintf("%s|%d", c.Subject, c.CommittedAt.Unix())
				if existingFingerprints[keyTime] || existingFingerprints[keySec] {
					isDup = true
				}
			}
		}

		if isDup {
			skippedCount++
		} else {
			toImport = append(toImport, c)
		}
	}

	if len(toImport) == 0 {
		ui.Successf("All %d git commit(s) already exist in project '%s' (skipped).", len(commits), local.ProjectSlug)
		if flagJSON {
			return outputJSON(ImportResult{
				Total:   len(commits),
				Skipped: skippedCount,
			})
		}
		return nil
	}

	// 4. Dry run preview
	if importDryRun {
		if flagJSON {
			return outputJSON(map[string]any{
				"dryRun":   true,
				"total":    len(commits),
				"skipped":  skippedCount,
				"toImport": len(toImport),
				"commits":  toImport,
			})
		}

		fmt.Println()
		ui.Infof("Dry run preview: found %d commits (%d skipped as existing, %d to import):",
			len(commits), skippedCount, len(toImport))
		fmt.Println()
		for i, c := range toImport {
			dateStr := ""
			if c.CommittedAt != nil {
				dateStr = c.CommittedAt.Format("2006-01-02 15:04")
			}
			authorStr := c.AuthorName
			if authorStr == "" {
				authorStr = c.AuthorEmail
			}
			fmt.Printf("  [%d/%d] %s  %s  %-15s  %s\n",
				i+1, len(toImport),
				ui.Yellow(ui.ShortID(c.Hash, 7)),
				ui.Dim(dateStr),
				ui.Cyan(ui.ShortID(authorStr, 15)),
				ui.Bold(c.Subject),
			)
		}
		msLabel := "none"
		if targetMilestoneId != "" {
			msLabel = targetMilestoneId
		}
		fmt.Println()
		fmt.Printf("Target Project: %s | Milestone: %s\n", ui.Bold(local.ProjectSlug), ui.Cyan(msLabel))
		ui.Successf("Dry run complete: %d records would be imported into project '%s' (no changes made).", len(toImport), local.ProjectSlug)
		return nil
	}

	// 5. Interactive confirmation
	if !importYes && ui.IsInteractive() && !flagJSON {
		fmt.Println()
		fmt.Printf("Target Project:    %s (%s)\n", ui.Bold(local.ProjectTitle), local.ProjectSlug)
		if targetMilestoneId != "" {
			fmt.Printf("Milestone:         %s\n", ui.Cyan(targetMilestoneId))
		}
		fmt.Printf("Commits to Import: %s (Total: %d, Skipped existing: %d)\n\n",
			ui.Green(strconv.Itoa(len(toImport))), len(commits), skippedCount)

		if !ui.PromptConfirm(fmt.Sprintf("Import %d commits into ProjectVitals?", len(toImport)), true) {
			ui.Info("Import cancelled.")
			return nil
		}
		fmt.Println()
	}

	// 6. Execute import
	ui.Infof("Importing %d commits to project '%s'...", len(toImport), local.ProjectSlug)

	var importedRecords []*api.Record
	var failedCount int

	for i, c := range toImport {
		if ctxErr := cmd.Context().Err(); ctxErr != nil {
			ui.Warningf("Import interrupted: %v", ctxErr)
			break
		}

		desc := buildCommitDescription(c, importIncludeHash, importIncludeAuthor)
		title := c.Subject
		if title == "" {
			title = "Git commit " + ui.ShortID(c.Hash, 7)
		}

		req := api.CreateRecordRequest{
			Title:       title,
			Description: desc,
			MilestoneID: targetMilestoneId,
			CommittedAt: c.CommittedAt,
		}

		rec, err := apiClient.CreateRecord(cmd.Context(), local.ProjectId, req)
		if err != nil {
			failedCount++
			if !flagJSON {
				fmt.Printf("  [%d/%d] %s %s: %s (%v)\n",
					i+1, len(toImport),
					ui.Red("✘"),
					ui.ShortID(c.Hash, 7),
					ui.ShortID(title, 40),
					err,
				)
			}
		} else {
			importedRecords = append(importedRecords, rec)
			if !flagJSON {
				fmt.Printf("  [%d/%d] %s %s: %s\n",
					i+1, len(toImport),
					ui.Green("✔"),
					ui.ShortID(c.Hash, 7),
					title,
				)
			}
		}
	}

	result := ImportResult{
		Total:    len(commits),
		Imported: len(importedRecords),
		Skipped:  skippedCount,
		Failed:   failedCount,
		Records:  importedRecords,
	}

	if flagJSON {
		return outputJSON(result)
	}

	fmt.Println()
	if failedCount > 0 {
		ui.Warningf("Import finished with issues: %d imported, %d skipped, %d failed (out of %d total commits).",
			len(importedRecords), skippedCount, failedCount, len(commits))
	} else {
		ui.Successf("Import completed successfully: %d imported, %d skipped (out of %d total commits).",
			len(importedRecords), skippedCount, len(commits))
	}

	return nil
}

func buildCommitDescription(c *git.CommitInfo, includeHash, includeAuthor bool) string {
	var parts []string
	if c.Body != "" {
		parts = append(parts, c.Body)
	}
	var meta []string
	if includeHash && c.Hash != "" {
		meta = append(meta, "Git-Commit: "+c.Hash)
	}
	if includeAuthor && c.AuthorName != "" {
		authorInfo := c.AuthorName
		if c.AuthorEmail != "" {
			authorInfo += " <" + c.AuthorEmail + ">"
		}
		meta = append(meta, "Git-Author: "+authorInfo)
	}
	if len(meta) > 0 {
		parts = append(parts, strings.Join(meta, "\n"))
	}
	return strings.Join(parts, "\n\n")
}

func fetchExistingRecordFingerprints(ctx context.Context, projectID string) map[string]bool {
	fingerprints := make(map[string]bool)
	page := 1
	for {
		resp, err := apiClient.ListRecordsWithFilter(ctx, projectID, "", "", page, 100)
		if err != nil {
			var apiErr *api.APIError
			if (errors.As(err, &apiErr) && apiErr.StatusCode == 404) || strings.Contains(err.Error(), "404") {
				break
			}
			ui.Warningf("Could not check existing records: %v (skipping duplicate check)", err)
			break
		}
		if resp == nil || len(resp.Data) == 0 {
			break
		}
		for _, rec := range resp.Data {
			// 1. Title + RFC3339 timestamp
			fingerprints[rec.Title+"|"+rec.CommittedAt.Format(time.RFC3339)] = true
			// 2. Title + Unix timestamp
			fingerprints[fmt.Sprintf("%s|%d", rec.Title, rec.CommittedAt.Unix())] = true

			// 3. Hash in description if present
			if rec.Description != nil && *rec.Description != "" {
				lines := strings.Split(*rec.Description, "\n")
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "Git-Commit:") {
						h := strings.TrimSpace(strings.TrimPrefix(trimmed, "Git-Commit:"))
						if h != "" {
							fingerprints["hash:"+h] = true
						}
					}
				}
			}
		}
		if resp.Meta == nil || page >= resp.Meta.TotalPages {
			break
		}
		page++
	}
	return fingerprints
}

func setupImportFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&importRev, "rev", "", "Git revision or range (e.g. HEAD, main, v1.0..HEAD)")
	cmd.Flags().StringVar(&importSince, "since", "", "Import commits newer than a specific date (e.g. '2026-01-01' or '2 weeks ago')")
	cmd.Flags().StringVar(&importUntil, "until", "", "Import commits older than a specific date")
	cmd.Flags().StringVar(&importAuthor, "author", "", "Filter commits by author name or email")
	cmd.Flags().IntVarP(&importLimit, "limit", "n", 0, "Maximum number of commits to import (0 for unlimited)")
	cmd.Flags().StringVar(&importMilestone, "milestone", "", "Milestone UUID to bind to imported records")
	cmd.Flags().BoolVar(&importSkipExisting, "skip-existing", true, "Skip git commits that already exist in the project")
	cmd.Flags().BoolVar(&importIncludeHash, "include-hash", false, "Append Git commit hash to record description")
	cmd.Flags().BoolVar(&importIncludeAuthor, "include-author", false, "Append Git author to record description")
	cmd.Flags().BoolVar(&importDryRun, "dry-run", false, "Preview commits to import without submitting to ProjectVitals")
	cmd.Flags().BoolVarP(&importYes, "yes", "y", false, "Skip interactive confirmation prompt")
	cmd.Flags().BoolVar(&importReverse, "reverse", true, "Import in chronological order (oldest to newest)")
	cmd.Flags().BoolVar(&importNoMerges, "no-merges", false, "Exclude merge commits from import")
}

func init() {
	setupImportFlags(importCmd)
	setupImportFlags(importGitCmd)

	importCmd.AddCommand(importGitCmd)
	rootCmd.AddCommand(importCmd)
}
