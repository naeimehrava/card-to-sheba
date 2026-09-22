package zarinhub

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/naeimehrava/card-to-sheba/backend/internal/inquiry"
)

type tokenSource struct {
	client   *Client
	username string
	password string
	static   string
	skew     time.Duration

	mu           sync.Mutex
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

func (t *tokenSource) Token(ctx context.Context) (string, error) {
	if t.static != "" {
		return t.static, nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.accessToken != "" && time.Now().Before(t.expiresAt.Add(-t.skew)) {
		return t.accessToken, nil
	}
	return t.fetchLocked(ctx)
}

func (t *tokenSource) Invalidate() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.accessToken = ""
}

func (t *tokenSource) fetchLocked(ctx context.Context) (string, error) {
	req := getTokenRequest{}

	if t.refreshToken != "" {
		req.RefreshToken = t.refreshToken
	} else {
		req.Username = t.username
		req.Password = t.password
	}

	data, _, err := doJSON[getTokenData](ctx, t.client, "/v5/Authentication/GetToken", req, "")
	if err != nil && t.refreshToken != "" {

		t.refreshToken = ""
		req = getTokenRequest{Username: t.username, Password: t.password}
		data, _, err = doJSON[getTokenData](ctx, t.client, "/v5/Authentication/GetToken", req, "")
	}
	if err != nil {
		return "", err
	}
	if data.AccessToken == "" {
		return "", inquiry.NewError(inquiry.KindUpstreamAuth,
			messageUpstreamAuth,
			errors.New("zarinhub returned an empty access token"))
	}

	ttl := time.Duration(data.ExpiresTn) * time.Second
	if ttl <= 0 {

		ttl = 5 * time.Minute
	}

	t.accessToken = data.AccessToken
	t.refreshToken = data.RefreshToken
	t.expiresAt = time.Now().Add(ttl)
	return t.accessToken, nil
}
