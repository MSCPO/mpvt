package auth

import (
	"bytes"
	"context"
	"crypto/rand"
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
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", validURL}
	case "darwin":
		cmd = "open"
		args = []string{validURL}
	default:
		cmd = "xdg-open"
		args = []string{validURL}
	}

	return exec.Command(cmd, args...).Start()
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

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(httpReq)
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
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>登录 - MSCPO ProjectVitals CLI</title>
  <script src="https://js.hcaptcha.com/1/api.js" async defer></script>
  <style>
    :root {
      --bg: #0d1117;
      --card-bg: #161b22;
      --border: #30363d;
      --text: #f0f6fc;
      --text-muted: #8b949e;
      --accent: #238636;
      --accent-hover: #2ea043;
      --error: #f85149;
      --error-bg: rgba(248, 81, 73, 0.15);
      --font: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
    }
    @media (prefers-color-scheme: light) {
      :root {
        --bg: #f6f8fa;
        --card-bg: #ffffff;
        --border: #d0d7de;
        --text: #1f2328;
        --text-muted: #656d76;
        --accent: #1f883d;
        --accent-hover: #1a7f37;
        --error: #cf222e;
        --error-bg: #ffebe9;
      }
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: var(--bg);
      color: var(--text);
      font-family: var(--font);
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 100vh;
      padding: 20px;
    }
    .card {
      background: var(--card-bg);
      border: 1px solid var(--border);
      border-radius: 12px;
      padding: 36px 32px;
      width: 100%%;
      max-width: 420px;
      box-shadow: 0 8px 24px rgba(0,0,0,0.15);
    }
    .logo-badge {
      display: inline-block;
      font-size: 13px;
      font-weight: 600;
      color: var(--accent);
      background: rgba(35, 134, 54, 0.15);
      padding: 4px 10px;
      border-radius: 20px;
      margin-bottom: 16px;
    }
    h1 {
      font-size: 22px;
      font-weight: 600;
      margin-bottom: 8px;
    }
    p.subtitle {
      font-size: 14px;
      color: var(--text-muted);
      margin-bottom: 24px;
    }
    .form-group {
      margin-bottom: 18px;
      text-align: left;
    }
    label {
      display: block;
      font-size: 13px;
      font-weight: 500;
      margin-bottom: 6px;
    }
    input[type="email"], input[type="password"] {
      width: 100%%;
      padding: 10px 12px;
      background: transparent;
      border: 1px solid var(--border);
      border-radius: 6px;
      color: var(--text);
      font-size: 14px;
      outline: none;
      transition: border-color 0.2s;
    }
    input[type="email"]:focus, input[type="password"]:focus {
      border-color: #58a6ff;
    }
    .captcha-wrap {
      display: flex;
      justify-content: center;
      margin: 20px 0 16px;
    }
    .btn {
      width: 100%%;
      padding: 11px;
      background: var(--accent);
      color: #fff;
      border: none;
      border-radius: 6px;
      font-size: 15px;
      font-weight: 600;
      cursor: pointer;
      transition: background 0.2s;
    }
    .btn:hover {
      background: var(--accent-hover);
    }
    .btn:disabled {
      opacity: 0.6;
      cursor: not-allowed;
    }
    .alert {
      padding: 10px 14px;
      border-radius: 6px;
      font-size: 13px;
      margin-bottom: 18px;
      display: none;
    }
    .alert-error {
      background: var(--error-bg);
      color: var(--error);
      border: 1px solid var(--error);
      display: block;
    }
    .success-card {
      text-align: center;
      display: none;
    }
    .success-icon {
      font-size: 48px;
      color: var(--accent);
      margin-bottom: 16px;
    }
  </style>
</head>
<body>
  <div class="card" id="loginCard">
    <div class="logo-badge">MSCPO ProjectVitals</div>
    <h1>开发者身份登录</h1>
    <p class="subtitle">登录后授权凭据将自动同步至终端 mpvt CLI</p>

    <div id="alertBox" class="alert"></div>

    <form id="loginForm">
      <div class="form-group">
        <label for="email">邮箱地址</label>
        <input type="email" id="email" placeholder="developer@mscpo.org" required autofocus />
      </div>

      <div class="form-group">
        <label for="password">密码</label>
        <input type="password" id="password" placeholder="请输入密码" required />
      </div>

      <div class="captcha-wrap">
        <div class="h-captcha" data-sitekey="%s" data-callback="onCaptchaVerified"></div>
      </div>

      <input type="hidden" id="csrfState" value="%s" />

      <button type="submit" class="btn" id="submitBtn">登录并授权 CLI</button>
    </form>
  </div>

  <div class="card success-card" id="successCard">
    <div class="success-icon">✔</div>
    <h1>登录成功！</h1>
    <p class="subtitle" id="userGreeting">欢迎回来</p>
    <p style="font-size: 14px; color: var(--text-muted); margin-top: 12px;">
      认证凭证已成功同步至终端，您可以关闭此浏览器标签页。
    </p>
  </div>

  <script>
    let captchaToken = '';

    window.onCaptchaVerified = function(token) {
      captchaToken = token;
      hideError();
    };

    function showError(msg) {
      const box = document.getElementById('alertBox');
      box.textContent = msg;
      box.className = 'alert alert-error';
    }

    function hideError() {
      const box = document.getElementById('alertBox');
      box.className = 'alert';
      box.textContent = '';
    }

    document.getElementById('loginForm').addEventListener('submit', async function(e) {
      e.preventDefault();
      hideError();

      const email = document.getElementById('email').value.trim();
      const password = document.getElementById('password').value;
      const state = document.getElementById('csrfState').value;

      if (!email || !password) {
        showError('请输入完整的邮箱与密码');
        return;
      }

      if (!captchaToken) {
        showError('请完成上方 hCaptcha 人机验证');
        return;
      }

      const btn = document.getElementById('submitBtn');
      btn.disabled = true;
      btn.textContent = '登录中...';

      try {
        const resp = await fetch('/submit', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ email, password, hCaptchaToken: captchaToken, state: state })
        });

        const data = await resp.json();

        if (!resp.ok || !data.success) {
          throw new Error(data.message || '登录失败，请检查账号密码');
        }

        // Show success screen
        document.getElementById('loginCard').style.display = 'none';
        const successCard = document.getElementById('successCard');
        successCard.style.display = 'block';

        const greeting = document.getElementById('userGreeting');
        if (data.user && data.user.email) {
          greeting.textContent = '欢迎回来，' + (data.user.name || data.user.email);
        }
      } catch (err) {
        showError(err.message);
        if (window.hcaptcha) {
          window.hcaptcha.reset();
        }
        captchaToken = '';
      } finally {
        btn.disabled = false;
        btn.textContent = '登录并授权 CLI';
      }
    });
  </script>
</body>
</html>`, siteKey, state)
}
