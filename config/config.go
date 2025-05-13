package config

import (
	"os"

	"github.com/jinzhu/configor"
)

type Config struct {
	Google    *GoogleConfig  `yaml:"google"`
	Twitter   *TwitterConfig `yaml:"twitter"`
	Instagram *InstaConfig   `yaml:"instagram"`
	Twitch    *TwitchConfig  `yaml:"twitch"`
	Redbook   *RedbookConfig `yaml:"redbook"`
	QNAP      *QNAPConfig    `yaml:"qnap"`

	Discord *DiscordConfig `yaml:"discord"`

	Redis  *RedisConfig `yaml:"redis"`
	Logger *LogConfig   `yaml:"logger"`
}

type GoogleConfig struct {
	APIKey string `yaml:"apiKey"`
}

type TwitterConfig struct {
	APIKey string `yaml:"apiKey"`
}

type InstaConfig struct {
	PostURLFormat  string `yaml:"postUrlFormat"`
	StoryURLFormat string `yaml:"storyUrlFormat"`
	UserURLFormat  string `yaml:"userUrlFormat"`
}

type TwitchConfig struct {
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
}

type RedbookConfig struct {
	PostURL string `yaml:"postUrl"`
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

	ProxyURL string `yaml:"proxyUrl"`
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

type LogConfig struct {
	LogPath string `yaml:"logPath"`
}

func (c *Config) LoadConfig(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}

	return configor.Load(c, path)
}
