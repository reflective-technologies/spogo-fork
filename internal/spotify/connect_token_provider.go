package spotify

import (
	"context"
	"errors"
	"time"
)

// connectTokenProvider exposes the already-established connect session access token
// through the generic TokenProvider interface used by the Web API client.
//
// This avoids additional /api/token calls (which can get 429'd) when we already have
// a valid access token from the connect session.
type connectTokenProvider struct {
	session *connectSession
}

func (p connectTokenProvider) Token(ctx context.Context) (Token, error) {
	if p.session == nil {
		return Token{}, errors.New("connect session not initialized")
	}
	auth, err := p.session.auth(ctx)
	if err != nil {
		return Token{}, err
	}
	// The connect session manages the real expiry; we only need a reasonable TTL to
	// keep the Web API client from refreshing too aggressively.
	return Token{
		AccessToken: auth.AccessToken,
		ExpiresAt:   time.Now().Add(30 * time.Minute),
	}, nil
}

