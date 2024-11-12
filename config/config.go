package config

import (
	"os"

	"github.com/jinzhu/configor"
)

type Config struct {
	Twitter   *TwitterConfig `yaml:"twitter"`
	Google    *GoogleConfig  `yaml:"google"`
	Instagram *InstaConfig   `yaml:"instagram"`
	Twitch    *TwitchConfig  `yaml:"twitch"`
	Tiktok    *TiktokConfig  `yaml:"tiktok"`
	QNAP      *QNAPConfig    `yaml:"qnap"`

	Discord *DiscordConfig `yaml:"discord"`

	Redis  *RedisConfig `yaml:"redis"`
	Logger *LogConfig   `yaml:"logger"`
}

type TwitterConfig struct {
	ConsumerKey    string `yaml:"consumerKey"`
	ConsumerSecret string `yaml:"consumerSecret"`
	AccessToken    string `yaml:"accessToken"`
	AccessSecret   string `yaml:"accessSecret"`

	PollIntervalMins        int      `yaml:"pollIntervalMins"`
	StalkerTimeoutMins      int      `yaml:"stalkerTimeoutMins"`
	MaxStreamRestartRetries int      `yaml:"maxStreamRestartRetries"`
	ExpandIgnoreRegexes     []string `yaml:"expandIgnoreRegexes"`
}

type GoogleConfig struct {
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

type TiktokConfig struct {
	VideoURLFormat string `yaml:"videoUrlFormat"`
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
