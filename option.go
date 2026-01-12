package chat_sdk

import "gorm.io/gorm"
import "github.com/go-redis/redis/v8"
import "time"

type ServiceConfig struct {
	Debug bool
}

type Config struct {
	DB          *gorm.DB
	RDB         *redis.Client
	TablePrefix string
	Service     ServiceConfig

	// GroupAvatarMerge 群头像合成配置（创建群时生成微信群风格拼图头像）
	GroupAvatarMerge GroupAvatarMergeConfig
}

// GroupAvatarMergeConfig 群头像合成配置（Engine级别）。
// OutputDir 为空时默认使用系统临时目录。
type GroupAvatarMergeConfig struct {
	Enabled    bool
	CanvasSize int
	Padding    int
	Gap        int
	Timeout    time.Duration
	OutputDir  string

	// URLPrefix 生成的群头像在业务上的访问路径前缀（写库用）。
	// 例："uploads/auto_avatar" 或 "/uploads/auto_avatar" 或 "https://cdn.xxx.com/uploads/auto_avatar"。
	// 为空时将使用 OutputDir（去掉 file:// 的逻辑已移除）。
	URLPrefix string
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

// WithGroupAvatarMergeConfig 配置群头像合成。
func WithGroupAvatarMergeConfig(cfg GroupAvatarMergeConfig) Option {
	return func(c *Config) {
		c.GroupAvatarMerge = cfg
	}
}
