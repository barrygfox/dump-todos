package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dump-todos-go/internal/config"
)

const authTimeout = 10 * time.Minute

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

type cachedToken struct {
	ClientID     string    `json:"client_id"`
	TenantID     string    `json:"tenant_id"`
	Scope        string    `json:"scope"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func AcquireToken(ctx context.Context, cfg config.Config) (string, error) {
	if cfg.UseTokenCache {
		cached, err := loadCachedToken(cfg)
		if err != nil {
			return "", err
		}
		if cached != nil {
			if cached.AccessToken != "" && time.Until(cached.ExpiresAt) > time.Minute {
				return cached.AccessToken, nil
			}
			if cached.RefreshToken != "" {
				refreshed, err := refreshToken(ctx, cfg, cached.RefreshToken)
				if err == nil {
					if err := saveCachedToken(cfg, refreshed); err != nil {
						return "", err
					}
					return refreshed.AccessToken, nil
				}
			}
		}
	}

	verifier, challenge, err := generatePKCE()
	if err != nil {
		return "", err
	}

	state, err := randomString(32)
	if err != nil {
		return "", err
	}

	code, err := authorizationCode(ctx, cfg, challenge, state)
	if err != nil {
		return "", err
	}

	token, err := exchangeCode(ctx, cfg, code, verifier)
	if err != nil {
		return "", err
	}
	if cfg.UseTokenCache {
		if err := saveCachedToken(cfg, token); err != nil {
			return "", err
		}
	}

	return token.AccessToken, nil
}

func authorizationCode(parent context.Context, cfg config.Config, challenge, expectedState string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, authTimeout)
	defer cancel()

	resultCh := make(chan struct {
		code string
		err  error
	}, 1)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.RedirectHost, cfg.RedirectPort),
		Handler: mux,
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if authErr := query.Get("error"); authErr != "" {
			http.Error(w, fmt.Sprintf("Error: %s\n%s", authErr, query.Get("error_description")), http.StatusBadRequest)
			select {
			case resultCh <- struct {
				code string
				err  error
			}{err: fmt.Errorf("auth error: %s", authErr)}:
			default:
			}
			return
		}

		state := query.Get("state")
		if state == "" || state != expectedState {
			http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
			select {
			case resultCh <- struct {
				code string
				err  error
			}{err: errors.New("invalid OAuth state")}:
			default:
			}
			return
		}

		code := query.Get("code")
		if code == "" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("Authentication successful! You can close this window."))
		select {
		case resultCh <- struct {
			code string
			err  error
		}{code: code}:
		default:
		}
	})

	listenErrCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErrCh <- err
		}
	}()

	authURL, err := authorizeURL(cfg, challenge, expectedState)
	if err != nil {
		_ = server.Shutdown(context.Background())
		return "", err
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Opening browser for authentication...")
	fmt.Fprintln(os.Stderr)

	if err := openBrowser(authURL); err != nil {
		fmt.Fprintf(os.Stderr, "Open this URL manually if needed:\n%s\n\n", authURL)
	}

	select {
	case result := <-resultCh:
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		_ = server.Shutdown(shutdownCtx)
		return result.code, result.err
	case err := <-listenErrCh:
		return "", fmt.Errorf("start redirect listener: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()
		_ = server.Shutdown(shutdownCtx)
		return "", errors.New("authentication timeout")
	}
}

func exchangeCode(ctx context.Context, cfg config.Config, code, verifier string) (tokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("client_id", cfg.ClientID)
	values.Set("code", code)
	values.Set("redirect_uri", cfg.RedirectURI())
	values.Set("code_verifier", verifier)
	values.Set("scope", cfg.Scope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL(), strings.NewReader(values.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tokenResponse{}, err
	}

	var parsed tokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return tokenResponse{}, err
	}
	if parsed.AccessToken != "" {
		return parsed, nil
	}
	if parsed.Error != "" {
		return tokenResponse{}, fmt.Errorf("token exchange failed: %s", parsed.Error)
	}
	return tokenResponse{}, fmt.Errorf("token exchange failed: unexpected response")
}

func refreshToken(ctx context.Context, cfg config.Config, refreshToken string) (tokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("client_id", cfg.ClientID)
	values.Set("refresh_token", refreshToken)
	values.Set("scope", cfg.Scope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL(), strings.NewReader(values.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tokenResponse{}, err
	}

	var parsed tokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return tokenResponse{}, err
	}
	if parsed.AccessToken != "" {
		if parsed.RefreshToken == "" {
			parsed.RefreshToken = refreshToken
		}
		return parsed, nil
	}
	if parsed.Error != "" {
		return tokenResponse{}, fmt.Errorf("refresh token failed: %s", parsed.Error)
	}
	return tokenResponse{}, fmt.Errorf("refresh token failed: unexpected response")
}

func authorizeURL(cfg config.Config, challenge, state string) (string, error) {
	u, err := url.Parse(cfg.AuthorizeURL())
	if err != nil {
		return "", err
	}

	query := u.Query()
	query.Set("client_id", cfg.ClientID)
	query.Set("response_type", "code")
	query.Set("redirect_uri", cfg.RedirectURI())
	query.Set("scope", cfg.Scope)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	query.Set("state", state)
	query.Set("prompt", "select_account")
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func openBrowser(target string) error {
	var command string
	switch runtime.GOOS {
	case "darwin":
		command = "open"
	case "linux":
		command = "xdg-open"
	case "windows":
		command = "rundll32"
	default:
		return fmt.Errorf("unsupported OS for browser launch: %s", runtime.GOOS)
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command(command, "url.dll,FileProtocolHandler", target)
	} else {
		cmd = exec.Command(command, target)
	}
	return cmd.Start()
}

func generatePKCE() (string, string, error) {
	verifier, err := randomString(32)
	if err != nil {
		return "", "", err
	}

	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])
	return verifier, challenge, nil
}

func randomString(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func loadCachedToken(cfg config.Config) (*cachedToken, error) {
	data, err := os.ReadFile(cfg.TokenCachePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read token cache: %w", err)
	}

	var cached cachedToken
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("parse token cache: %w", err)
	}
	if cached.ClientID != cfg.ClientID || cached.TenantID != cfg.TenantID || cached.Scope != cfg.Scope {
		return nil, nil
	}
	return &cached, nil
}

func saveCachedToken(cfg config.Config, token tokenResponse) error {
	if token.AccessToken == "" {
		return errors.New("cannot cache an empty access token")
	}

	cache := cachedToken{
		ClientID:     cfg.ClientID,
		TenantID:     cfg.TenantID,
		Scope:        cfg.Scope,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(token.ExpiresIn) * time.Second),
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize token cache: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.TokenCachePath), 0o700); err != nil {
		return fmt.Errorf("create token cache directory: %w", err)
	}
	if err := os.WriteFile(cfg.TokenCachePath, data, 0o600); err != nil {
		return fmt.Errorf("write token cache: %w", err)
	}
	return nil
}
