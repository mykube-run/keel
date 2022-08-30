package database

import (
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/types"
)

func New(conf config.DatabaseConfig) (types.DB, error) {
	switch conf.Type {
	case "mock":
		return NewMockDB(), nil
	case "mysql":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported database type: %v", conf.Type)
	}
}
