// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mpvt/internal/api"
	"mpvt/internal/config"
)

func TestStatus_UnboundReturnsError(t *testing.T) {
	activeConfig = &config.Config{Local: nil}
	err := statusCmd.RunE(statusCmd, []string{})
	if err == nil {
		t.Fatal("expected error when unbound, got nil")
	}
}

func TestStatus_WithMockAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		desc := "A platform for developers"
		json.NewEncoder(w).Encode(api.ApiResponse[api.Project]{
			Data: api.Project{
				Id:          "proj-status",
				Title:       "Status Tester",
				Slug:        "status-tester",
				Status:      "IN_DEVELOPMENT",
				Visibility:  "PUBLIC",
				Description: &desc,
			},
		})
	})
	mux.HandleFunc("/api/v1/projects/proj-status/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.Record]{
			Data: []api.Record{
				{
					Id:          "rec-01",
					Title:       "First commit",
					CommittedAt: time.Now(),
				},
			},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local: &config.LocalConfig{
			ProjectId:             "proj-status",
			ProjectSlug:           "status-tester",
			DefaultMilestoneId:    "ms-1",
			DefaultMilestoneTitle: "Sprint 1",
		},
	}
	apiClient = api.NewClient(activeConfig)

	// Run text output mode
	if err := statusCmd.RunE(statusCmd, []string{}); err != nil {
		t.Fatalf("statusCmd text mode failed: %v", err)
	}

	// Run JSON mode
	flagJSON = true
	defer func() { flagJSON = false }()
	if err := statusCmd.RunE(statusCmd, []string{}); err != nil {
		t.Fatalf("statusCmd JSON mode failed: %v", err)
	}
}
