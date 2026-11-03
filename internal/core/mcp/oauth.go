package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"github.com/pkg/browser"
	"golang.org/x/oauth2"

	"larder/internal/core/model"
	"larder/internal/platform"
)

type oauthSession struct {
	Config oauth2.Config `json:"config"`
	Token  oauth2.Token  `json:"token"`
}

func oauthAccount(endpoint string) string {
	sum := sha256.Sum256([]byte(endpoint))
	return "mcp-oauth:" + hex.EncodeToString(sum[:12])
}

func loadOAuthSession(endpoint string) *oauthSession {
	raw, err := platform.GetKeychainSecret(oauthAccount(endpoint))
	if err != nil {
		return nil
	}
	var session oauthSession
	if json.Unmarshal([]byte(raw), &session) != nil || session.Config.ClientID == "" {
		return nil
	}
	return &session
}

func saveOAuthSession(endpoint string, config *oauth2.Config, token *oauth2.Token) {
	raw, err := json.Marshal(oauthSession{Config: *config, Token: *token})
	if err != nil {
		return
	}
	_ = platform.SetKeychainSecret(oauthAccount(endpoint), string(raw))
}

// newOAuthHandler prepares SDK-managed OAuth without starting the flow. The
// browser/callback server only starts if the transport receives a 401/403.
func newOAuthHandler(endpoint string) (sdkauth.OAuthHandler, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	redirectURL := "http://" + listener.Addr().String() + "/callback"
	callback := make(chan sdkauth.AuthorizationResult, 1)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		code, state := query.Get("code"), query.Get("state")
		if code == "" {
			http.Error(w, "authorization was not completed", http.StatusBadRequest)
			return
		}
		_, _ = fmt.Fprintln(w, "Larder is now connected to the MCP server. You can close this tab.")
		server.Close()
		callback <- sdkauth.AuthorizationResult{Code: code, State: state}
	})
	go func() { _ = server.Serve(listener) }()
	cleanup := func() { _ = server.Close() }
	config := &sdkauth.AuthorizationCodeHandlerConfig{
		RedirectURL: redirectURL,
		DynamicClientRegistrationConfig: &sdkauth.DynamicClientRegistrationConfig{Metadata: &oauthex.ClientRegistrationMetadata{
			RedirectURIs: []string{redirectURL}, ClientName: "Larder",
			GrantTypes:    []string{"authorization_code", "refresh_token"},
			ResponseTypes: []string{"code"}, TokenEndpointAuthMethod: "none",
		}},
		RequestRefreshToken: true,
		Client:              &http.Client{Timeout: 30 * time.Second},
		AuthorizationCodeFetcher: func(ctx context.Context, args *sdkauth.AuthorizationArgs) (*sdkauth.AuthorizationResult, error) {
			if err := browser.OpenURL(args.URL); err != nil {
				return nil, err
			}
			select {
			case result := <-callback:
				return &result, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}
	if session := loadOAuthSession(endpoint); session != nil {
		config.InitialTokenSource = session.Config.TokenSource(context.Background(), &session.Token)
	}
	config.NewTokenSource = func(ctx context.Context, config *oauth2.Config, token *oauth2.Token) (oauth2.TokenSource, error) {
		saveOAuthSession(endpoint, config, token)
		return config.TokenSource(ctx, token), nil
	}
	handler, err := sdkauth.NewAuthorizationCodeHandler(config)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return handler, cleanup, nil
}

// OAuthStatus reports whether a token is stored for this endpoint.
func OAuthStatus(endpoint string) model.MCPAuthStatus {
	session := loadOAuthSession(endpoint)
	if session == nil {
		return model.MCPAuthStatus{}
	}
	status := model.MCPAuthStatus{Configured: true, HasRefresh: session.Token.RefreshToken != ""}
	if !session.Token.Expiry.IsZero() {
		status.ExpiresAt = session.Token.Expiry.UTC().Format(time.RFC3339)
	}
	return status
}

// ClearOAuth removes the stored token and client registration for an endpoint.
func ClearOAuth(endpoint string) error {
	return platform.DeleteKeychainSecret(oauthAccount(endpoint))
}
