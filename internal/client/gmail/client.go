package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

	callbackAddr     = ":8080"
	callbackEndpoint = "/google/callback"
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
	config  *oauth2.Config
	token   *oauth2.Token
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
	client := &Client{
		config: config,
	}
	if err := client.registerService(ctx); err != nil {
		return nil, err
	}
	return client, nil
}

// Client surfaces the gmail service for client interactions.
func (c *Client) Client() *gmail.Service {
	return c.service
}

func (c *Client) registerService(ctx context.Context) error {
	if err := c.registerToken(ctx); err != nil {
		return fmt.Errorf("fetch auth token: %w", err)
	}
	client := c.config.Client(ctx, c.token)
	service, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("create gmail service: %w", err)
	}
	c.service = service
	return nil
}

// registerToken retrieves an auth token either from local cache, or
// from web. If retrieved from web, it also auto-caches the token
// to a local file.
func (c *Client) registerToken(ctx context.Context) error {
	if err := c.registerTokenFromFile(); err == nil {
		return nil
	}
	if err := c.registerTokenFromWeb(ctx, c.config); err != nil {
		return fmt.Errorf("retrieve token from web: %w", err)
	}
	if err := cacheToken(c.token); err != nil {
		return fmt.Errorf("cache token: %w", err)
	}
	return nil
}

// registerTokenFromFile retrieves a token from a local file.
func (c *Client) registerTokenFromFile() error {
	f, err := os.Open(tokenFile)
	if err != nil {
		return err
	}
	defer f.Close()
	token := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(token); err != nil {
		return err
	}
	c.token = token
	return nil
}

// registerTokenFromWeb requests a token from the web, and
// registers the retrieved token.
func (c *Client) registerTokenFromWeb(ctx context.Context, config *oauth2.Config) error {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	errCh := make(chan error, 1)
	go func() {
		http.HandleFunc(callbackEndpoint, c.handleCallback(ctx, errCh))

		http.ListenAndServe(callbackAddr, nil)
	}()
	if err := <-errCh; err != nil {
		return fmt.Errorf("handle auth callback: %w", err)
	}
	return nil
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

func (c *Client) handleCallback(ctx context.Context, errCh chan<- error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		authCode := r.FormValue("code")
		token, err := c.config.Exchange(ctx, authCode)
		if err != nil {
			errCh <- err
			return
		}
		c.token = token
		errCh <- nil
	}
}
