package twitch

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/nicklaw5/helix"
	"github.com/xIceArcher/go-leah/config"
)

type API struct{}

var (
	api          *helix.Client
	apiSetupOnce sync.Once
)

var ErrNotFound = errors.New("resource not found")

func NewAPI(cfg *config.TwitchConfig) (API, error) {
	var err error
	apiSetupOnce.Do(func() {
		if api, err = helix.NewClient(&helix.Options{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
		}); err != nil {
			return
		}

		var resp *helix.AppAccessTokenResponse
		resp, err = api.RequestAppAccessToken([]string{})
		if err != nil {
			return
		}

		api.SetAppAccessToken(resp.Data.AccessToken)
	})

	return API{}, err
}

func (API) GetStream(loginName string) (*helix.Stream, error) {
	streams, err := api.GetStreams(&helix.StreamsParams{
		UserLogins: []string{loginName},
	})
	if err != nil {
		return nil, err
	}
	if len(streams.Data.Streams) == 0 {
		return nil, ErrNotFound
	}

	return &streams.Data.Streams[0], nil
}

func (API) GetUser(loginName string) (*helix.User, error) {
	users, err := api.GetUsers(&helix.UsersParams{
		Logins: []string{loginName},
	})
	if err != nil {
		return nil, err
	}
	if len(users.Data.Users) == 0 {
		return nil, ErrNotFound
	}

	return &users.Data.Users[0], nil
}

func (API) FormatThumbnailURL(url string, width int, height int) string {
	url = strings.ReplaceAll(url, "{width}", strconv.Itoa(width))
	return strings.ReplaceAll(url, "{height}", strconv.Itoa(height))
}

func (API) GetUserURL(loginName string) string {
	return fmt.Sprintf("https://twitch.tv/%s", loginName)
}
