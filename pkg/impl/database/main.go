package database

import (
	"fmt"
	"github.com/mykube-run/keel/pkg/config"
	"github.com/mykube-run/keel/pkg/impl/database/mock"
	"github.com/mykube-run/keel/pkg/impl/database/mongodb"
	"github.com/mykube-run/keel/pkg/impl/database/mongodb_redis"
	"github.com/mykube-run/keel/pkg/impl/database/mysql"
	"github.com/mykube-run/keel/pkg/impl/database/mysql_redis"
	"github.com/mykube-run/keel/pkg/types"
	"strings"
)

func New(conf config.DatabaseConfig) (types.DB, error) {
	switch strings.ToLower(conf.Type) {
	case "mock":
		return mock.NewMockDB(), nil
	case "mysql":
		return mysql.New(conf.DSN)
	case "mysql+redis":
		return mysql_redis.New(conf.DSN)
	case "mongodb":
		return mongodb.New(conf.DSN)
	case "mongodb+redis":
		return mongodb_redis.New(conf.DSN)
	default:
		return nil, fmt.Errorf("unsupported database type: %v", conf.Type)
	}
}
