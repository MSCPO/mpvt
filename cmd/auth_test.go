// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"mpvt/internal/api"
	"mpvt/internal/config"
)

func TestAuthStatus_WithMockHealth(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(api.HealthResponse{
			Status:  "healthy",
			Service: "project-vitals",
			Version: "1.0.0",
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	activeConfig = &config.Config{
		Global: config.GlobalConfig{
			Endpoint:  server.URL,
			Token:     "test-bearer-token-12345",
			UserEmail: "dev@mscpo.org",
			UserName:  "Developer",
			UserRole:  "USER",
		},
	}
	apiClient = api.NewClient(activeConfig)

	// 1. Plain text format
	if err := authStatusCmd.RunE(authStatusCmd, []string{}); err != nil {
		t.Fatalf("authStatusCmd failed: %v", err)
	}

	// 2. JSON format with redaction
	flagJSON = true
	defer func() { flagJSON = false }()
	if err := authStatusCmd.RunE(authStatusCmd, []string{}); err != nil {
		t.Fatalf("authStatusCmd JSON mode failed: %v", err)
	}
}

func TestAuthLogin_ManualTokenSavesConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpHome, ".config"))

	activeConfig = &config.Config{}
	loginToken = "my-manual-token-secret"
	loginUserId = "u-123"
	loginUserEmail = "user@test.org"
	loginUserRole = "USER"
	defer func() {
		loginToken = ""
		loginUserId = ""
		loginUserEmail = ""
		loginUserRole = ""
	}()

	if err := authLoginCmd.RunE(authLoginCmd, []string{}); err != nil {
		t.Fatalf("authLoginCmd manual token failed: %v", err)
	}

	if activeConfig.Global.Token != "my-manual-token-secret" {
		t.Errorf("expected token to be saved, got %q", activeConfig.Global.Token)
	}
}
