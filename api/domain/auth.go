package domain

import (
	"context"
	"fmt"
	"time"
)

type AuthToken struct {
	AccessToken string
	TokenType   string
	ExpiresIn   time.Duration
}

func (m *AuthToken) MarshalJSON() ([]byte, error) {
	return []byte(
		fmt.Sprintf(
			`{"access_token":"%s","token_type":"%s","expires_in":%d}`,
			m.AccessToken,
			m.TokenType,
			int(m.ExpiresIn.Seconds()))), nil
}

type AuthCache interface {
	GetTokenExpiry() time.Duration
	GetEmailByToken(ctx context.Context, token string) (string, error)
	GenerateAndSaveToken(ctx context.Context, email string) (string, error)
	DeleteToken(ctx context.Context, token string) error
}
