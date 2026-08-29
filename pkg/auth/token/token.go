package token

import "context"

type Verifier interface {
	VerifyToken(ctx context.Context, token string) (ID string, err error)
}
