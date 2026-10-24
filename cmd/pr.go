package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"mpvt/internal/api"
	"mpvt/internal/ui"
)

var (
	prCreateTitle       string
	prCreateDescription string
	prCreatePatchFile   string
	prStatusFilter      string
)

var prCmd = &cobra.Command{
	Use:     "pr",
	Aliases: []string{"proposal"},
	Short:   "Manage concept modification proposals (like Git PR in PLANNING stage)",
}

var prListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List proposals for current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		local, err := activeConfig.EnsureProjectBound()
		if err != nil {
			return err
		}

		proposals, err := apiClient.ListProposals(cmd.Context(), local.ProjectId, prStatusFilter)
		if err != nil {
			return fmt.Errorf("failed to fetch proposals: %w", err)
		}

		if flagJSON {
			return outputJSON(proposals)
		}

		if len(proposals) == 0 {
			ui.Info("No proposals found for current project.")
			return nil
		}

		table := ui.NewTable("ID", "Status", "Author", "Title", "Created At")

		for _, p := range proposals {
			shortID := ui.ShortID(p.Id, 8)
			authorName := "Anonymous"
			if p.Author != nil {
				authorName = p.Author.DisplayName()
			}

			table.Append(
				shortID,
				ui.FormatProposalBadge(p.Status),
				authorName,
				p.Title,
				p.CreatedAt.Format("2006-01-02 15:04"),
			)
		}

		table.Render()
		return nil
	},
}

var prCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Submit a new concept modification proposal (PR)",
	RunE: func(cmd *cobra.Command, args []string) error {
		local, err := activeConfig.EnsureProjectBound()
		if err != nil {
			return err
		}

		title := prCreateTitle
		if title == "" {
			var err error
			title, err = ui.PromptText("Proposal Title", "")
			if err != nil {
				return err
			}
		}
		if title == "" {
			return fmt.Errorf("title cannot be empty")
		}

		desc := prCreateDescription
		if desc == "" {
			var err error
			desc, err = ui.PromptMultiline("Proposal Description")
			if err != nil {
				return err
			}
		}

		var patchData api.PatchData
		if prCreatePatchFile != "" {
			info, err := os.Stat(prCreatePatchFile)
			if err != nil {
				return fmt.Errorf("failed to stat patch file %s: %w", prCreatePatchFile, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("patch file %s must be a regular file", prCreatePatchFile)
			}
			if info.Size() > 10*1024*1024 {
				return fmt.Errorf("patch file %s exceeds maximum allowed size of 10MB", prCreatePatchFile)
			}

			fileData, err := os.ReadFile(prCreatePatchFile)
			if err != nil {
				return fmt.Errorf("failed to read patch file %s: %w", prCreatePatchFile, err)
			}
			if err := json.Unmarshal(fileData, &patchData); err != nil {
				return fmt.Errorf("invalid patch JSON: %w", err)
			}
			if len(patchData.Blocks) > 1000 {
				return fmt.Errorf("patch file %s contains %d blocks, exceeding maximum allowed limit of 1000", prCreatePatchFile, len(patchData.Blocks))
			}
		} else {
			// Fetch existing concept doc
			currentDoc, err := apiClient.GetConcept(cmd.Context(), local.ProjectId)
			if err != nil {
				return fmt.Errorf("failed to fetch current concept document: %w", err)
			}

			fmt.Println()
			ui.Infof("Current concept doc has %d block(s).", len(currentDoc.Blocks))
			newBlockText, err := ui.PromptMultiline("Enter text for new concept section to propose")
			if err != nil {
				return err
			}
			if newBlockText == "" {
				return fmt.Errorf("no concept content provided")
			}

			newBlock := api.ConceptBlock{
				Id:   currentDoc.NextBlockID(),
				Type: "paragraph",
				Text: newBlockText,
			}

			patchData = api.PatchData{
				Blocks:  append(currentDoc.Blocks, newBlock),
				Summary: title,
			}
		}

		ui.Infof("Submitting proposal to project %s ...", local.ProjectSlug)
		created, err := apiClient.CreateProposal(cmd.Context(), local.ProjectId, title, desc, patchData)
		if err != nil {
			return fmt.Errorf("failed to create proposal: %w", err)
		}

		if flagJSON {
			return outputJSON(created)
		}

		ui.Successf("Proposal submitted! ID: %s", ui.Yellow(created.Id))
		return nil
	},
}

var prMergeCmd = &cobra.Command{
	Use:   "merge <proposal-id>",
	Short: "Merge an open proposal into concept document (Project Admin/Owner only)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		proposalId := args[0]
		if err := apiClient.MergeProposal(cmd.Context(), proposalId); err != nil {
			return fmt.Errorf("failed to merge proposal: %w", err)
		}

		ui.Successf("Proposal %s merged successfully!", proposalId)
		return nil
	},
}

var prCloseCmd = &cobra.Command{
	Use:   "close <proposal-id>",
	Short: "Close or reject an open proposal",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		proposalId := args[0]
		if err := apiClient.CloseProposal(cmd.Context(), proposalId); err != nil {
			return fmt.Errorf("failed to close proposal: %w", err)
		}

		ui.Successf("Proposal %s closed.", proposalId)
		return nil
	},
}

func init() {
	prListCmd.Flags().StringVar(&prStatusFilter, "status", "", "Filter by status: OPEN, MERGED, CLOSED")

	prCreateCmd.Flags().StringVarP(&prCreateTitle, "title", "t", "", "Proposal title")
	prCreateCmd.Flags().StringVarP(&prCreateDescription, "description", "d", "", "Proposal description")
	prCreateCmd.Flags().StringVarP(&prCreatePatchFile, "file", "f", "", "JSON patch file containing {blocks: [...]}")

	prCmd.AddCommand(prListCmd)
	prCmd.AddCommand(prCreateCmd)
	prCmd.AddCommand(prMergeCmd)
	prCmd.AddCommand(prCloseCmd)

	rootCmd.AddCommand(prCmd)
}
