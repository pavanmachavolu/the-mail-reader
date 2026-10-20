package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	microsoftGraphURL = "https://graph.microsoft.com/v1.0"
	scopes            = "offline_access Mail.Read User.Read"
)

var microsoftOAuthURL = func() string {
	tenant := os.Getenv("MAILREADER_TENANT")
	if tenant == "" {
		tenant = "consumers"
	}
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0", tenant)
}()

var redirectURI = func() string {
	if v := os.Getenv("MAILREADER_REDIRECT_URI"); v != "" {
		return v
	}
	return "http://localhost:8080/callback"
}()

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

func getConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".the-mail-reader", "config.json")
}

func loadConfig() (*Config, error) {
	configPath := getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var config Config
	err = json.Unmarshal(data, &config)
	return &config, err
}

func saveConfig(config *Config) error {
	configPath := getConfigPath()
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0600)
}

func generateState() string {
	return uuid.New().String()
}

func generateVerifier() string {
	return uuid.New().String() + uuid.New().String()
}

func s256Challenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func buildAuthURL(clientID, state, verifier string) string {
	params := url.Values{
		"client_id":             {clientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {scopes},
		"state":                 {state},
		"code_challenge":        {s256Challenge(verifier)},
		"code_challenge_method": {"S256"},
	}

	return fmt.Sprintf("%s/authorize?%s", microsoftOAuthURL, params.Encode())
}

func ExchangeCode(code, verifier, clientID, clientSecret string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}

	return postTokenRequest(data)
}

func RefreshAccessToken(config *Config) error {
	if config.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {config.ClientID},
		"refresh_token": {config.RefreshToken},
		"scope":         {scopes},
	}
	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}

	tokenResp, err := postTokenRequest(data)
	if err != nil {
		return err
	}

	config.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		config.RefreshToken = tokenResp.RefreshToken
	}
	config.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return saveConfig(config)
}

type oauthError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func postTokenRequest(data url.Values) (*TokenResponse, error) {
	resp, err := http.Post(
		fmt.Sprintf("%s/token", microsoftOAuthURL),
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	if tokenResp.AccessToken == "" {
		var oerr oauthError
		if err := json.Unmarshal(body, &oerr); err == nil && oerr.Error != "" {
			return nil, fmt.Errorf("token endpoint returned %s: %s", oerr.Error, oerr.ErrorDescription)
		}
		return nil, fmt.Errorf("token endpoint returned http %d", resp.StatusCode)
	}

	return &tokenResp, nil
}

func StartCallbackServer(state string) (string, error) {
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	listenAddr := ":8080"
	if u, err := url.Parse(redirectURI); err == nil && u.Port() != "" {
		listenAddr = ":" + u.Port()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "Invalid state", http.StatusBadRequest)
			errCh <- fmt.Errorf("invalid state parameter")
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "No code received", http.StatusBadRequest)
			authErr := r.URL.Query().Get("error")
			authDesc := r.URL.Query().Get("error_description")
			if authErr == "" {
				authErr = "no authorization code received"
			}
			if authDesc != "" {
				errCh <- fmt.Errorf("%s: %s", authErr, authDesc)
			} else {
				errCh <- fmt.Errorf("%s", authErr)
			}
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `
			<!DOCTYPE html>
			<html>
			<head><title>Authentication Complete</title></head>
			<body style="font-family: -apple-system, sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background: #f5f5f5;">
				<div style="text-align: center; padding: 40px; background: white; border-radius: 12px; box-shadow: 0 2px 10px rgba(0,0,0,0.1);">
					<h1 style="color: #0078d4;">✓ Authentication Successful</h1>
					<p>You can close this window and return to the terminal.</p>
				</div>
			</body>
			</html>
		`)

		codeCh <- code
	})

	server := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case code := <-codeCh:
		server.Shutdown(context.Background())
		return code, nil
	case err := <-errCh:
		server.Shutdown(context.Background())
		return "", err
	case <-time.After(5 * time.Minute):
		server.Shutdown(context.Background())
		return "", fmt.Errorf("authentication timeout")
	}
}
