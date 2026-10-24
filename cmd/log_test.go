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

func TestLogCmd_UnboundReturnsError(t *testing.T) {
	activeConfig = &config.Config{Local: nil}
	if err := logCmd.RunE(logCmd, []string{}); err == nil {
		t.Fatal("expected error when project is unbound, got nil")
	}
}

func TestLogCmd_WithMockAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-log/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		desc := "Multiline\nDetailed\nCommit description"
		msTitle := "Alpha Milestone"
		author := &api.User{
			ID:    "u-1",
			Email: "alice@example.com",
		}
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.Record]{
			Data: []api.Record{
				{
					Id:          "rec-001",
					Title:       "Implemented caching",
					Description: &desc,
					Author:      author,
					Milestone: &api.Milestone{
						Id:    "ms-1",
						Title: msTitle,
					},
					CommittedAt: time.Now(),
				},
			},
			Meta: &api.Meta{
				Page:       1,
				TotalPages: 2,
				Total:      15,
			},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local: &config.LocalConfig{
			ProjectId:   "proj-log",
			ProjectSlug: "log-proj",
		},
	}
	apiClient = api.NewClient(activeConfig)

	// 1. Default multi-line format
	logOneline = false
	if err := logCmd.RunE(logCmd, []string{}); err != nil {
		t.Fatalf("logCmd failed: %v", err)
	}

	// 2. Oneline format
	logOneline = true
	defer func() { logOneline = false }()
	if err := logCmd.RunE(logCmd, []string{}); err != nil {
		t.Fatalf("logCmd oneline mode failed: %v", err)
	}

	// 3. JSON format
	flagJSON = true
	defer func() { flagJSON = false }()
	if err := logCmd.RunE(logCmd, []string{}); err != nil {
		t.Fatalf("logCmd JSON mode failed: %v", err)
	}
}

func TestLogCmd_EmptyResults(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/projects/proj-empty-log/records", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.ApiResponse[[]api.Record]{
			Data: []api.Record{},
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{Endpoint: server.URL},
		Local:  &config.LocalConfig{ProjectId: "proj-empty-log"},
	}
	apiClient = api.NewClient(activeConfig)

	if err := logCmd.RunE(logCmd, []string{}); err != nil {
		t.Fatalf("logCmd with empty results failed: %v", err)
	}
}
