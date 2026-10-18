package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"mpvt/internal/ui"
)

var blameCmd = &cobra.Command{
	Use:     "blame",
	Aliases: []string{"concept"},
	Short:   "View concept document with block-level contributor attribution (like Git Blame)",
	RunE: func(cmd *cobra.Command, args []string) error {
		local, err := activeConfig.EnsureProjectBound()
		if err != nil {
			return err
		}

		doc, err := apiClient.GetConceptDocBlame(cmd.Context(), local.ProjectId)
		if err != nil {
			return fmt.Errorf("failed to fetch concept blame: %w", err)
		}

		if flagJSON {
			return outputJSON(doc)
		}

		fmt.Printf("● Concept Document for %s (Version %d)\n", ui.Bold(local.ProjectTitle), doc.Version)
		fmt.Println("─────────────────────────────────────────────────────────────────")

		if len(doc.Blocks) == 0 {
			ui.Info("Concept document has no text blocks yet.")
			return nil
		}

		for i, block := range doc.Blocks {
			contributor := "Initial Author / Owner"
			var prInfo string
			if block.Attribution != nil {
				if block.Attribution.Contributor != nil {
					contributor = block.Attribution.Contributor.DisplayName()
				}
				if block.Attribution.ProposalTitle != nil && *block.Attribution.ProposalTitle != "" {
					prInfo = fmt.Sprintf("via PR: %s", *block.Attribution.ProposalTitle)
				}
			}

			fmt.Printf("%s %s %s\n",
				ui.Cyan(fmt.Sprintf("[%02d %s]", i+1, block.Id)),
				ui.Yellow(contributor),
				ui.Dim(prInfo),
			)
			fmt.Printf("   %s\n\n", block.Text)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(blameCmd)
}
