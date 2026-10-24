// SPDX-FileCopyrightText: 2026 CyanTea <ushio_noa@disroot.org>
// SPDX-FileCopyrightText: 2026 Minecraft Server Collective Promotion Organization
//
// SPDX-License-Identifier: MPL-2.0

package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

//go:embed login.html
var loginHTMLTemplate string

const (
	DefaultHCaptchaSiteKey = "c3a057be-fd54-428a-8908-d98825741c5a"
	DefaultAuthBaseURL     = "https://api.mscpo.com"
)

type AuthUser struct {
	Id        string  `json:"id"`
	Email     string  `json:"email"`
	Name      *string `json:"name,omitempty"`
	AvatarUrl *string `json:"avatarUrl,omitempty"`
	Role      string  `json:"role"`
	Status    string  `json:"status,omitempty"`
}

type LoginResult struct {
	User         AuthUser `json:"user"`
	AccessToken  string   `json:"accessToken"`
	RefreshToken string   `json:"refreshToken"`
}

type loginSubmitRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	HCaptchaToken string `json:"hCaptchaToken"`
	State         string `json:"state"`
}

var openBrowserFn = OpenBrowser

// OpenBrowser opens the URL in the user's default browser after validating the scheme
func OpenBrowser(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme %q, must be http or https", parsed.Scheme)
	}
	if strings.HasPrefix(parsed.Host, "-") {
		return fmt.Errorf("invalid hostname: %s", parsed.Host)
	}

	validURL := parsed.String()
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", "", validURL}
	case "darwin":
		cmd = "open"
		args = []string{validURL}
	default:
		cmd = "xdg-open"
		args = []string{validURL}
	}

	c := exec.Command(cmd, args...)
	if err := c.Start(); err != nil {
		return err
	}
	go func() { _ = c.Wait() }()
	return nil
}

func generateStateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate csrf token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func validateHost(host string, port int) bool {
	expected1 := fmt.Sprintf("127.0.0.1:%d", port)
	expected2 := fmt.Sprintf("localhost:%d", port)
	return host == expected1 || host == expected2
}

func buildAuthHandler(ctx context.Context, authBaseURL, siteKey, expectedState string, port int, resultChan chan<- *LoginResult) http.Handler {
	mux := http.NewServeMux()

	escapedSiteKey := html.EscapeString(siteKey)
	escapedState := html.EscapeString(expectedState)

	proxyClient := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	// 1. Serve Login Webpage
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://hcaptcha.com https://*.hcaptcha.com; frame-src https://hcaptcha.com https://*.hcaptcha.com; style-src 'self' 'unsafe-inline'; frame-ancestors 'none';")
		htmlContent := renderLoginPage(escapedSiteKey, escapedState)
		w.Write([]byte(htmlContent))
	})

	// 2. Handle Login Submission
	mux.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Validate Content-Type
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" && !strings.HasPrefix(ct, "application/json;") {
			writeJSONError(w, http.StatusForbidden, "Invalid Content-Type")
			return
		}

		var req loginSubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
			return
		}

		// Validate CSRF state token
		if req.State == "" || req.State != expectedState {
			writeJSONError(w, http.StatusForbidden, "Invalid CSRF state token")
			return
		}

		if req.Email == "" || req.Password == "" {
			writeJSONError(w, http.StatusBadRequest, "请输入邮箱和密码")
			return
		}
		if req.HCaptchaToken == "" {
			writeJSONError(w, http.StatusBadRequest, "请先完成 hCaptcha 人机验证")
			return
		}

		// Forward login request to AuthServerApi
		authEndpoint := authBaseURL + "/api/v1/auth/login"
		reqBody, _ := json.Marshal(map[string]string{
			"email":         req.Email,
			"password":      req.Password,
			"hCaptchaToken": req.HCaptchaToken,
		})

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, authEndpoint, bytes.NewReader(reqBody))
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")

		resp, err := proxyClient.Do(httpReq)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, "无法连接到认证服务器: "+err.Error())
			return
		}
		defer resp.Body.Close()

		respData, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))

		if resp.StatusCode != http.StatusOK {
			var errObj struct {
				Message string `json:"message"`
				Error   any    `json:"error"`
			}
			json.Unmarshal(respData, &errObj)

			msg := "登录失败"
			if errObj.Message != "" {
				msg = errObj.Message
			} else if strErr, ok := errObj.Error.(string); ok && strErr != "" {
				msg = strErr
			} else if mapErr, ok := errObj.Error.(map[string]any); ok {
				if errMsg, ok := mapErr["message"].(string); ok && errMsg != "" {
					msg = errMsg
				}
			}
			writeJSONError(w, resp.StatusCode, msg)
			return
		}

		var successResp struct {
			Data LoginResult `json:"data"`
		}
		if err := json.Unmarshal(respData, &successResp); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "解析登录响应失败")
			return
		}

		// Success!
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"user":    successResp.Data.User,
		})

		// Send to channel non-blocking (LEAK-01 / SEC-08)
		select {
		case resultChan <- &successResp.Data:
		default:
		}
	})

	// Wrap mux with DNS rebinding protection (SEC-04)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !validateHost(r.Host, port) {
			http.Error(w, "Forbidden: Invalid Host header", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// StartBrowserLogin starts a temporary local HTTP server to handle login with hCaptcha
func StartBrowserLogin(ctx context.Context, authBaseURL, siteKey string) (*LoginResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if authBaseURL == "" {
		authBaseURL = DefaultAuthBaseURL
	}
	authBaseURL = strings.TrimRight(authBaseURL, "/")

	parsedAuthURL, err := url.Parse(authBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid auth URL %q: %w", authBaseURL, err)
	}
	isLocal := parsedAuthURL.Hostname() == "127.0.0.1" || parsedAuthURL.Hostname() == "localhost"
	if parsedAuthURL.Scheme != "https" && !isLocal {
		return nil, fmt.Errorf("insecure auth URL %q: HTTPS is required for authentication endpoints", authBaseURL)
	}

	if siteKey == "" {
		siteKey = DefaultHCaptchaSiteKey
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start local auth listener: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	localURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	expectedState, err := generateStateToken()
	if err != nil {
		return nil, err
	}

	resultChan := make(chan *LoginResult, 1)
	errChan := make(chan error, 1)

	handler := buildAuthHandler(ctx, authBaseURL, siteKey, expectedState, port, resultChan)

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	fmt.Println()
	fmt.Println("正在启动本地认证服务...")
	fmt.Printf("🌐 请在浏览器中完成登录与人机验证：\n   %s\n\n", localURL)

	// Attempt to open browser automatically
	if err := openBrowserFn(localURL); err != nil {
		fmt.Printf("无法自动启动浏览器 (%v)，请手动在浏览器中复制并打开上述链接。\n", err)
	} else {
		fmt.Println("已自动唤起系统浏览器，等待验证完成...")
	}

	// Wait for result, cancellation, or timeout (5 minutes)
	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return nil, ctx.Err()

	case res := <-resultChan:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return res, nil

	case err := <-errChan:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return nil, fmt.Errorf("local auth server error: %w", err)

	case <-timer.C:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return nil, fmt.Errorf("login timed out after 5 minutes")
	}
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"message": msg,
	})
}

func renderLoginPage(siteKey, state string) string {
	content := strings.Replace(loginHTMLTemplate, "{{SITE_KEY}}", siteKey, 1)
	return strings.Replace(content, "{{STATE}}", state, 1)
}
