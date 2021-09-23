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

	Discord *DiscordConfig `yaml:"discord"`

	Logger *zap.Config `yaml:"logger"`
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

type DiscordConfig struct {
	Token   string `yaml:"token"`
	Prefix  string `yaml:"prefix"`
	AdminID string `yaml:"adminID"`

	Commands map[string]*DiscordCommandConfig `yaml:"commands"`
	Handlers map[string]*DiscordHandlerConfig `yaml:"handlers"`

	FilterRegexes []string `yaml:"filterRegexes"`
}

type DiscordCommandConfig struct {
	IsAdminOnly bool `yaml:"isAdminOnly"`
}

type DiscordHandlerConfig struct {
	Regexes []string `yaml:"regexes"`
}

func (c *Config) LoadConfig(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}

	return configor.Load(c, path)
}
