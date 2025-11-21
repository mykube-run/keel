package types

import "context"

type AuthProvider interface {
    Authenticate(ctx context.Context, headers map[string]string) error
}