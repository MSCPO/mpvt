package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestBrowserLoginMock(t *testing.T) {
	// Mock MSCPO Auth Server
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/login" {
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)

			if body["email"] == "test@mscpo.org" && body["password"] == "correctpass" && body["hCaptchaToken"] != "" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"accessToken":  "mock-jwt-token-abc",
						"refreshToken": "mock-refresh-token-xyz",
						"user": map[string]any{
							"id":    "user-100",
							"email": "test@mscpo.org",
							"name":  "Test Developer",
							"role":  "USER",
						},
					},
				})
				return
			}

			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]string{"message": "Invalid email or password"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer authServer.Close()

	// Verify AuthUser struct and LoginResult parsing
	resJSON := `{"accessToken":"mock-jwt-token-abc","refreshToken":"mock-refresh-token-xyz","user":{"id":"user-100","email":"test@mscpo.org","name":"Test Developer","role":"USER"}}`
	var loginRes LoginResult
	if err := json.Unmarshal([]byte(resJSON), &loginRes); err != nil {
		t.Fatalf("failed to unmarshal LoginResult: %v", err)
	}

	if loginRes.AccessToken != "mock-jwt-token-abc" {
		t.Errorf("expected mock-jwt-token-abc, got %s", loginRes.AccessToken)
	}
	if loginRes.User.Email != "test@mscpo.org" {
		t.Errorf("expected test@mscpo.org, got %s", loginRes.User.Email)
	}

	// Verify mock post to auth server
	reqBody, _ := json.Marshal(map[string]string{
		"email":         "test@mscpo.org",
		"password":      "correctpass",
		"hCaptchaToken": "mock-hcaptcha-token",
	})
	resp, err := http.Post(authServer.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST to mock auth failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", resp.StatusCode)
	}
}

func TestStartBrowserLogin_ContextCancellation(t *testing.T) {
	origOpener := openBrowserFn
	openBrowserFn = func(string) error { return nil }
	defer func() { openBrowserFn = origOpener }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Pre-cancel

	res, err := StartBrowserLogin(ctx, "http://127.0.0.1:9999", "test-key")
	if res != nil {
		t.Errorf("expected nil result on canceled context, got %+v", res)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestStartBrowserLogin_HTMLEscaping(t *testing.T) {
	xssKey := `"><script>alert('xss')</script>`
	state := "1234567890abcdef"
	escapedKey := html.EscapeString(xssKey)
	escapedState := html.EscapeString(state)

	page := renderLoginPage(escapedKey, escapedState)
	if strings.Contains(page, "<script>alert('xss')</script>") {
		t.Error("page contains unescaped XSS payload")
	}
	if !strings.Contains(page, html.EscapeString(xssKey)) {
		t.Error("page does not contain properly escaped siteKey")
	}
	if !strings.Contains(page, state) {
		t.Error("page does not contain CSRF state")
	}
}

func TestHostValidation_DNSRebinding(t *testing.T) {
	port := 8080
	if !validateHost("127.0.0.1:8080", port) {
		t.Error("expected 127.0.0.1:8080 to be allowed")
	}
	if !validateHost("localhost:8080", port) {
		t.Error("expected localhost:8080 to be allowed")
	}
	if validateHost("attacker.com:8080", port) {
		t.Error("expected attacker.com to be rejected")
	}
	if validateHost("localhost:9090", port) {
		t.Error("expected different port to be rejected")
	}
	if validateHost("evil.com", port) {
		t.Error("expected evil.com to be rejected")
	}
}

func TestBuildAuthHandler_SecurityChecks(t *testing.T) {
	expectedState := "secure-csrf-token-1234"
	port := 8080
	resultChan := make(chan *LoginResult, 1)

	// Mock auth server that responds with valid login
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": LoginResult{
				AccessToken:  "mock-access-token",
				RefreshToken: "mock-refresh-token",
				User: AuthUser{
					Id:    "user-1",
					Email: "user@test.org",
					Role:  "USER",
				},
			},
		})
	}))
	defer authServer.Close()

	handler := buildAuthHandler(context.Background(), authServer.URL, "test-key", expectedState, port, resultChan)

	// 1. Host validation test - invalid host header
	reqBadHost := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/", nil)
	reqBadHost.Host = "rebind.attacker.com:8080"
	recBadHost := httptest.NewRecorder()
	handler.ServeHTTP(recBadHost, reqBadHost)
	if recBadHost.Code != http.StatusForbidden {
		t.Errorf("expected 403 for rebinding host, got %d", recBadHost.Code)
	}

	// 2. Host validation test - valid host header
	reqGoodHost := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/", nil)
	reqGoodHost.Host = "127.0.0.1:8080"
	recGoodHost := httptest.NewRecorder()
	handler.ServeHTTP(recGoodHost, reqGoodHost)
	if recGoodHost.Code != http.StatusOK {
		t.Errorf("expected 200 for valid host, got %d", recGoodHost.Code)
	}

	// 3. Content-Type validation test - invalid Content-Type
	reqBadCT := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/submit", strings.NewReader(`{}`))
	reqBadCT.Host = "127.0.0.1:8080"
	reqBadCT.Header.Set("Content-Type", "text/plain")
	recBadCT := httptest.NewRecorder()
	handler.ServeHTTP(recBadCT, reqBadCT)
	if recBadCT.Code != http.StatusForbidden {
		t.Errorf("expected 403 for invalid Content-Type, got %d", recBadCT.Code)
	}

	// 4. CSRF state token validation - wrong state
	submitBadState := loginSubmitRequest{
		Email:         "user@test.org",
		Password:      "mypassword",
		HCaptchaToken: "token123",
		State:         "wrong-csrf-token",
	}
	bodyBadState, _ := json.Marshal(submitBadState)
	reqBadState := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/submit", bytes.NewReader(bodyBadState))
	reqBadState.Host = "127.0.0.1:8080"
	reqBadState.Header.Set("Content-Type", "application/json")
	recBadState := httptest.NewRecorder()
	handler.ServeHTTP(recBadState, reqBadState)
	if recBadState.Code != http.StatusForbidden {
		t.Errorf("expected 403 for wrong CSRF state, got %d", recBadState.Code)
	}

	// 5. Successful submission with valid CSRF token and valid host
	submitGood := loginSubmitRequest{
		Email:         "user@test.org",
		Password:      "mypassword",
		HCaptchaToken: "token123",
		State:         expectedState,
	}
	bodyGood, _ := json.Marshal(submitGood)
	reqGood := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/submit", bytes.NewReader(bodyGood))
	reqGood.Host = "127.0.0.1:8080"
	reqGood.Header.Set("Content-Type", "application/json")
	recGood := httptest.NewRecorder()
	handler.ServeHTTP(recGood, reqGood)
	if recGood.Code != http.StatusOK {
		t.Errorf("expected 200 for valid submission, got %d", recGood.Code)
	}

	select {
	case res := <-resultChan:
		if res.AccessToken != "mock-access-token" {
			t.Errorf("expected mock-access-token, got %s", res.AccessToken)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for resultChan")
	}
}

func TestStartBrowserLogin_EndToEnd(t *testing.T) {
	// Mock Auth Server
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": LoginResult{
				AccessToken:  "e2e-token-xyz",
				RefreshToken: "e2e-refresh-xyz",
				User: AuthUser{
					Id:    "user-e2e",
					Email: "e2e@test.org",
					Role:  "USER",
				},
			},
		})
	}))
	defer authServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	origOpener := openBrowserFn
	defer func() { openBrowserFn = origOpener }()

	// Simulate browser opening and submitting form with extracted CSRF state
	openBrowserFn = func(url string) error {
		go func() {
			// Fetch the login page to extract CSRF token
			resp, err := http.Get(url)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			bodyBytes, _ := io.ReadAll(resp.Body)
			bodyStr := string(bodyBytes)

			// Extract CSRF state from input hidden field
			re := regexp.MustCompile(`id="csrfState" value="([^"]+)"`)
			matches := re.FindStringSubmatch(bodyStr)
			if len(matches) < 2 {
				return
			}
			csrfState := matches[1]

			// Send valid POST /submit
			submitReq := loginSubmitRequest{
				Email:         "e2e@test.org",
				Password:      "password",
				HCaptchaToken: "valid-captcha",
				State:         csrfState,
			}
			reqBytes, _ := json.Marshal(submitReq)

			submitResp, err := http.Post(url+"/submit", "application/json", bytes.NewReader(reqBytes))
			if err == nil {
				submitResp.Body.Close()
			}
		}()
		return nil
	}

	res, err := StartBrowserLogin(ctx, authServer.URL, "test-site-key")
	if err != nil {
		t.Fatalf("StartBrowserLogin failed: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil LoginResult")
	}
	if res.AccessToken != "e2e-token-xyz" {
		t.Errorf("expected e2e-token-xyz, got %s", res.AccessToken)
	}
	if res.User.Email != "e2e@test.org" {
		t.Errorf("expected e2e@test.org, got %s", res.User.Email)
	}
}

func TestOpenBrowserValidation(t *testing.T) {
	invalidURLs := []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"ftp://example.com",
		"http://-bad-host.com",
		"://invalid-url",
	}

	for _, u := range invalidURLs {
		err := OpenBrowser(u)
		if err == nil {
			t.Errorf("expected error for invalid URL %q, got nil", u)
		}
	}
}

