package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mpvt/internal/api"
	"mpvt/internal/config"
)

func TestPRList_WithMockAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-pr/proposals", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.Proposal]{
			Data: []api.Proposal{
				{
					Id:        "pr-01",
					Title:     "Add API Docs",
					Status:    "OPEN",
					CreatedAt: time.Now(),
				},
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local:  &config.LocalConfig{ProjectId: "proj-pr", ProjectSlug: "pr-proj"},
	}
	apiClient = api.NewClient(activeConfig)

	if err := prListCmd.RunE(prListCmd, []string{}); err != nil {
		t.Fatalf("prListCmd failed: %v", err)
	}

	flagJSON = true
	defer func() { flagJSON = false }()
	if err := prListCmd.RunE(prListCmd, []string{}); err != nil {
		t.Fatalf("prListCmd JSON mode failed: %v", err)
	}
}

func TestPRCreate_PatchBlockLimit(t *testing.T) {
	tmpDir := t.TempDir()
	patchFile := filepath.Join(tmpDir, "huge_patch.json")

	// Generate a patch with > 1000 blocks to test SEC-004 limit
	hugeBlocks := make([]api.ConceptBlock, 1005)
	for i := range hugeBlocks {
		hugeBlocks[i] = api.ConceptBlock{Id: "b-test", Type: "paragraph", Text: "text"}
	}
	patchData := api.PatchData{
		Blocks: hugeBlocks,
	}
	data, _ := json.Marshal(patchData)
	if err := os.WriteFile(patchFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	activeConfig = &config.Config{
		Local: &config.LocalConfig{ProjectId: "proj-pr"},
	}

	prCreateTitle = "Test Huge Patch"
	prCreateDescription = "Test Description"
	prCreatePatchFile = patchFile
	defer func() {
		prCreateTitle = ""
		prCreateDescription = ""
		prCreatePatchFile = ""
	}()

	err := prCreateCmd.RunE(prCreateCmd, []string{})
	if err == nil {
		t.Fatal("expected error when patch has > 1000 blocks, got nil")
	}
}

func TestPRMergeAndClose(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/proposals/pr-merge-01/merge", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": true}`))
	})
	mux.HandleFunc("/api/v1/proposals/pr-close-01/close", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": true}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
	}
	apiClient = api.NewClient(activeConfig)

	if err := prMergeCmd.RunE(prMergeCmd, []string{"pr-merge-01"}); err != nil {
		t.Fatalf("prMergeCmd failed: %v", err)
	}

	if err := prCloseCmd.RunE(prCloseCmd, []string{"pr-close-01"}); err != nil {
		t.Fatalf("prCloseCmd failed: %v", err)
	}
}
