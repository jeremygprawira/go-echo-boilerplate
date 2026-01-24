package openauth

import "go-echo-boilerplate/internal/config"

type OAuth struct {
	Google *GoogleOAuth
}

func Initialize(config *config.Configuration) (*OAuth, error) {
	googleOAuth, err := InitializeGoogleOAuth(config)
	if err != nil {
		return nil, err
	}

	return &OAuth{
		Google: googleOAuth,
	}, nil
}
