package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

var (
	ErrInvalidOptions = errors.New("invalid options")
)

const (
	// tokenFile stores user access and refresh tokens, and is
	// created automatically when the auth flow completes the
	// first time.
	tokenFile = "token.json"
)

// Options is used to configure a new gmail client.
type Options struct {
	// CredFilePath is the path to the Google
	// Dev Console client_credentials.json.
	CredFilePath string
}

// Client instruments a GMail client.
type Client struct {
	service *gmail.Service
}

// NewClient creates a new gmail Client.
func NewClient(ctx context.Context, opts *Options) (*Client, error) {
	if opts == nil {
		return nil, fmt.Errorf("%w: nil opts", ErrInvalidOptions)
	}
	cred, err := os.ReadFile(opts.CredFilePath)
	if err != nil {
		return nil, fmt.Errorf("read cred file: %w", err)
	}
	config, err := google.ConfigFromJSON(cred, gmail.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("build google config: %w", err)
	}
	token, err := fetchToken(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("fetch auth token: %w", err)
	}
	client := config.Client(ctx, token)
	service, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("create gmail service: %w", err)
	}
	return &Client{
		service: service,
	}, nil
}

// Client surfaces the gmail service for client interactions.
func (c *Client) Client() *gmail.Service {
	return c.service
}

// fetchToken retrieves an auth token either from local cache, or
// from web. If retrieved from web, it also auto-caches the token
// to a local file.
func fetchToken(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	if token, err := tokenFromFile(); err == nil {
		return token, nil
	}
	token, err := tokenFromWeb(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("retrieve token from web: %w", err)
	}
	if err := cacheToken(token); err != nil {
		return nil, fmt.Errorf("cache token: %w", err)
	}
	return token, nil
}

// tokenFromFile retrieves a token from a local file.
func tokenFromFile() (*oauth2.Token, error) {
	f, err := os.Open(tokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(tok); err != nil {
		return nil, err
	}
	return tok, nil
}

// tokenFromWeb requests a token from the web, and returns the
// retrieved token.
func tokenFromWeb(ctx context.Context, config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %w", err)
	}
	token, err := config.Exchange(ctx, authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %w", err)
	}
	return token, nil
}

// cacheToken saves a valid token to local file.
func cacheToken(token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", tokenFile)
	f, err := os.OpenFile(tokenFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %w", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}
