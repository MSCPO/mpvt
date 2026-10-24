package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mpvt/internal/api"
	"mpvt/internal/config"
)

func TestBlameCmd_UnboundReturnsError(t *testing.T) {
	activeConfig = &config.Config{Local: nil}
	if err := blameCmd.RunE(blameCmd, []string{}); err == nil {
		t.Fatal("expected error when project is unbound, got nil")
	}
}

func TestBlameCmd_WithMockAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-blame/concept/blame", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		prTitle := "PR #1: System Architecture"
		json.NewEncoder(w).Encode(api.ApiResponse[api.ConceptDoc]{
			Data: api.ConceptDoc{
				Version: 2,
				Blocks: []api.ConceptBlock{
					{
						Id:   "b-1",
						Type: "paragraph",
						Text: "Initial system overview paragraph.",
						Attribution: &api.AttributionRecord{
							Contributor: &api.User{
								ID:    "u-alice",
								Email: "alice@mscpo.org",
							},
							ProposalTitle: &prTitle,
						},
					},
					{
						Id:   "b-2",
						Type: "paragraph",
						Text: "Second block with nil attribution.",
					},
				},
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local: &config.LocalConfig{
			ProjectId:    "proj-blame",
			ProjectTitle: "Blame Project",
		},
	}
	apiClient = api.NewClient(activeConfig)

	// 1. Text format
	if err := blameCmd.RunE(blameCmd, []string{}); err != nil {
		t.Fatalf("blameCmd failed: %v", err)
	}

	// 2. JSON format
	flagJSON = true
	defer func() { flagJSON = false }()
	if err := blameCmd.RunE(blameCmd, []string{}); err != nil {
		t.Fatalf("blameCmd JSON mode failed: %v", err)
	}
}

func TestBlameCmd_EmptyBlocks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-empty-blame/concept/blame", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[api.ConceptDoc]{
			Data: api.ConceptDoc{Version: 1, Blocks: []api.ConceptBlock{}},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local:  &config.LocalConfig{ProjectId: "proj-empty-blame"},
	}
	apiClient = api.NewClient(activeConfig)

	if err := blameCmd.RunE(blameCmd, []string{}); err != nil {
		t.Fatalf("blameCmd with empty blocks failed: %v", err)
	}
}
