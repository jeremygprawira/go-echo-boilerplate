package firebaseclient

import (
	"context"

	"go-echo-boilerplate/internal/config"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

// Client wraps a Firebase App and derives Firestore/Auth clients from it.
type Client struct {
	app *firebase.App
	fs  *firestore.Client
}

func New(cfg config.Firebase) (*Client, error) {
	ctx := context.Background()
	opts := []option.ClientOption{}
	if cfg.CredentialsFile != "" {
		opts = append(opts, option.WithAuthCredentialsFile(option.ServiceAccount, cfg.CredentialsFile))
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: cfg.ProjectID}, opts...)
	if err != nil {
		return nil, err
	}
	fs, err := app.Firestore(ctx)
	if err != nil {
		return nil, err
	}
	return &Client{app: app, fs: fs}, nil
}

func (c *Client) Firestore() *firestore.Client { return c.fs }
func (c *Client) Close() error                 { return c.fs.Close() }
