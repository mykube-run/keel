package auth

import (
    "context"
    "errors"
    "strings"
)

type SimpleProvider struct {
    keys map[string]struct{}
}

func NewSimpleProvider(keys []string) *SimpleProvider {
    m := make(map[string]struct{})
    for _, k := range keys {
        k = strings.TrimSpace(k)
        if k != "" {
            m[k] = struct{}{}
        }
    }
    return &SimpleProvider{keys: m}
}

func (p *SimpleProvider) Authenticate(ctx context.Context, headers map[string]string) error {
    key := strings.TrimSpace(headers["x-api-key"])
    if _, ok := p.keys[key]; !ok {
        return errors.New("unauthenticated")
    }
    return nil
}