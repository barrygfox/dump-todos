package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const (
	defaultClientID     = "3187224c-ea09-4c7f-94bc-b1ba83001a4e"
	defaultTenantID     = "518a43e5-ff84-49ea-9a28-73053588b03d"
	defaultScope        = "Tasks.Read offline_access"
	defaultRedirectHost = "127.0.0.1"
	defaultRedirectPort = 3000
)

type Config struct {
	ClientID       string
	TenantID       string
	Scope          string
	RedirectHost   string
	RedirectPort   int
	OutputPath     string
	TokenCachePath string
	UseTokenCache  bool
	IncompleteOnly bool
}

func Load() (Config, error) {
	defaultPort, err := envInt("DUMP_TODOS_REDIRECT_PORT", defaultRedirectPort)
	if err != nil {
		return Config{}, err
	}

	defaultCachePath, err := tokenCachePath()
	if err != nil {
		return Config{}, err
	}

	clientID := flag.String("client-id", envString("DUMP_TODOS_CLIENT_ID", defaultClientID), "Microsoft Entra application client ID")
	tenantID := flag.String("tenant-id", envString("DUMP_TODOS_TENANT_ID", defaultTenantID), "Microsoft Entra tenant ID")
	scope := flag.String("scope", envString("DUMP_TODOS_SCOPE", defaultScope), "OAuth scope list")
	redirectHost := flag.String("redirect-host", envString("DUMP_TODOS_REDIRECT_HOST", defaultRedirectHost), "OAuth redirect listener host")
	redirectPort := flag.Int("redirect-port", defaultPort, "OAuth redirect listener port")
	outputPath := flag.String("output", envString("DUMP_TODOS_OUTPUT", ""), "Output markdown file; defaults to stdout when omitted")
	tokenCache := flag.String("token-cache", envString("DUMP_TODOS_TOKEN_CACHE", defaultCachePath), "Path to the token cache file")
	noTokenCache := flag.Bool("no-token-cache", false, "Disable token caching and force interactive authentication")
	incompleteOnly := flag.Bool("incomplete", false, "Export only incomplete tasks")

	flag.Parse()

	if *clientID == "" {
		return Config{}, fmt.Errorf("client ID is required")
	}
	if *tenantID == "" {
		return Config{}, fmt.Errorf("tenant ID is required")
	}
	if *scope == "" {
		return Config{}, fmt.Errorf("scope is required")
	}
	if *redirectHost == "" {
		return Config{}, fmt.Errorf("redirect host is required")
	}
	if *redirectPort <= 0 || *redirectPort > 65535 {
		return Config{}, fmt.Errorf("redirect port must be between 1 and 65535")
	}
	if !*noTokenCache && *tokenCache == "" {
		return Config{}, fmt.Errorf("token cache path is required unless --no-token-cache is set")
	}

	return Config{
		ClientID:       *clientID,
		TenantID:       *tenantID,
		Scope:          *scope,
		RedirectHost:   *redirectHost,
		RedirectPort:   *redirectPort,
		OutputPath:     *outputPath,
		TokenCachePath: *tokenCache,
		UseTokenCache:  !*noTokenCache,
		IncompleteOnly: *incompleteOnly,
	}, nil
}

func (c Config) RedirectURI() string {
	return fmt.Sprintf("http://%s:%d", c.RedirectHost, c.RedirectPort)
}

func (c Config) AuthorizeURL() string {
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", c.TenantID)
}

func (c Config) TokenURL() string {
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", c.TenantID)
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func tokenCachePath() (string, error) {
	if override := os.Getenv("DUMP_TODOS_TOKEN_CACHE"); override != "" {
		return override, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	return filepath.Join(configDir, "dump-todos-go", "token-cache.json"), nil
}
