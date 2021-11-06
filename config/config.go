package config

import (
	"os"

	"github.com/jinzhu/configor"
	"go.uber.org/zap"
)

type Config struct {
	Twitter   *TwitterConfig `yaml:"twitter"`
	Google    *GoogleConfig  `yaml:"google"`
	Instagram *InstaConfig   `yaml:"instagram"`
	Twitch    *TwitchConfig  `yaml:"twitch"`
	QNAP      *QNAPConfig    `yaml:"qnap"`

	Discord *DiscordConfig `yaml:"discord"`

	Redis  *RedisConfig `yaml:"redis"`
	Logger *zap.Config  `yaml:"logger"`
}

type TwitterConfig struct {
	ConsumerKey    string `yaml:"consumerKey"`
	ConsumerSecret string `yaml:"consumerSecret"`
	AccessToken    string `yaml:"accessToken"`
	AccessSecret   string `yaml:"accessSecret"`

	MaxStreamRestartRetries int      `yaml:"maxStreamRestartRetries" default:"5"`
	ExpandIgnoreRegexes     []string `yaml:"expandIgnoreRegexes"`
}

type GoogleConfig struct {
	APIKey string `yaml:"apiKey"`
}

type InstaConfig struct {
	PostURLFormat string `yaml:"postUrlFormat"`
}

type TwitchConfig struct {
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
}

type QNAPConfig struct {
	IsEnabled        bool   `yaml:"isEnabled"`
	URL              string `yaml:"url"`
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
	DownloadBasePath string `yaml:"downloadBasePath"`
}

type DiscordConfig struct {
	Token   string `yaml:"token"`
	Prefix  string `yaml:"prefix"`
	AdminID string `yaml:"adminID"`

	Cogs     map[string]*DiscordCogConfig     `yaml:"cogs"`
	Handlers map[string]*DiscordHandlerConfig `yaml:"handlers"`

	FilterRegexes []string `yaml:"filterRegexes"`
}

type DiscordCogConfig struct {
	IsAdminOnly bool     `yaml:"isAdminOnly"`
	Commands    []string `yaml:"commands"`
	ChannelIDs  []string `yaml:"channelIDs"`
}

type DiscordHandlerConfig struct {
	Regexes []string `yaml:"regexes"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

func (c *Config) LoadConfig(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}

	return configor.Load(c, path)
}
