package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"mpvt/internal/ui"
)

var (
	logLimit     int
	logPage      int
	logMilestone string
	logAuthor    string
	logOneline   bool
)

var logCmd = &cobra.Command{
	Use:     "log",
	Aliases: []string{"history"},
	Short:   "Display progress commit logs",
	Long:    `Browse the timeline of non-code progress commits submitted to the linked project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		local, err := activeConfig.EnsureProjectBound()
		if err != nil {
			return err
		}

		resp, err := apiClient.ListRecordsWithFilter(cmd.Context(), local.ProjectId, logMilestone, logAuthor, logPage, logLimit)
		if err != nil {
			return fmt.Errorf("failed to retrieve records: %w", err)
		}

		if flagJSON {
			return outputJSON(resp)
		}

		if len(resp.Data) == 0 {
			ui.Info("No progress records found for this query.")
			return nil
		}

		for _, r := range resp.Data {
			shortID := ui.ShortID(r.Id, 8)

			authorStr := "Unknown"
			if r.Author != nil {
				authorStr = r.Author.DisplayName()
			}

			msStr := ""
			if r.Milestone != nil {
				msStr = fmt.Sprintf("[%s]", r.Milestone.Title)
			}

			if logOneline {
				fmt.Printf("%s %s %s %s - %s\n",
					ui.Yellow(shortID),
					ui.Dim(r.CommittedAt.Format("2006-01-02 15:04")),
					ui.Cyan(msStr),
					ui.Dim(authorStr),
					ui.Bold(r.Title),
				)
			} else {
				fmt.Printf("%s %s\n", ui.Yellow("commit"), ui.Yellow(r.Id))
				fmt.Printf("Author:    %s\n", authorStr)
				fmt.Printf("Date:      %s\n", r.CommittedAt.Format("Mon Jan 02 15:04:05 2006 -0700"))
				if msStr != "" {
					fmt.Printf("Milestone: %s\n", ui.Cyan(msStr))
				}
				fmt.Println()
				fmt.Printf("    %s\n", ui.Bold(r.Title))
				if r.Description != nil && *r.Description != "" {
					descLines := strings.Split(*r.Description, "\n")
					for _, line := range descLines {
						fmt.Printf("    %s\n", line)
					}
				}
				fmt.Println()
			}
		}

		if resp.Meta != nil && resp.Meta.TotalPages > 1 {
			fmt.Printf("%s Page %d of %d (Total %d records). Use --page to navigate.\n",
				ui.Dim("───"),
				resp.Meta.Page,
				resp.Meta.TotalPages,
				resp.Meta.Total,
			)
		}

		return nil
	},
}

func init() {
	logCmd.Flags().IntVarP(&logLimit, "limit", "n", 10, "Number of commits to display")
	logCmd.Flags().IntVar(&logPage, "page", 1, "Page number")
	logCmd.Flags().StringVar(&logMilestone, "milestone", "", "Filter by milestone ID")
	logCmd.Flags().StringVar(&logAuthor, "author", "", "Filter by author UUID")
	logCmd.Flags().BoolVar(&logOneline, "oneline", false, "Display each record on a single line")

	rootCmd.AddCommand(logCmd)
}
