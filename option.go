package chat_sdk

import "gorm.io/gorm"
import "github.com/go-redis/redis/v8"

type ServiceConfig struct {
	Debug bool
}

type Config struct {
	DB          *gorm.DB
	RDB         *redis.Client
	TablePrefix string
	Service     ServiceConfig
}

type Option func(*Config)

func WithDB(db *gorm.DB) Option {
	return func(c *Config) {
		c.DB = db
	}
}

func WithTablePrefix(prefix string) Option {
	return func(c *Config) {
		c.TablePrefix = prefix
	}
}

func WithRDB(RDB *redis.Client) Option {
	return func(c *Config) {
		c.RDB = RDB
	}
}

func WithServiceDebug(debug bool) Option {
	return func(c *Config) {
		c.Service.Debug = debug
	}
}
