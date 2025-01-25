package redbook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xIceArcher/go-leah/config"
)

type API struct {
	url string

	client *http.Client
}

func NewAPI(cfg *config.RedbookConfig) (*API, error) {
	return &API{
		url: cfg.PostURL,

		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (a *API) GetPost(postURL string) (*Post, error) {
	reqBytes, err := json.Marshal(&RawRedbookRequest{
		URL: postURL,
	})
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Post(a.url, "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	rawResp := &RawRedbookResponse{}
	if err := json.Unmarshal(body, rawResp); err != nil {
		return nil, err
	}

	ret := &Post{
		ID:          rawResp.Data.ID,
		URL:         rawResp.URL,
		Title:       rawResp.Data.Title,
		Description: rawResp.CleanDescription(),
		CreateTime:  rawResp.CreateTime(),
		Author: &Author{
			ID:   rawResp.Data.AuthorID,
			URL:  rawResp.Data.AuthorURL,
			Name: rawResp.Data.AuthorNickname,
		},
	}

	switch rawResp.Data.Type {
	case RawRedbookMediaTypePhoto:
		ret.PhotoURLs = rawResp.Data.MediaDownloadURLs
	case RawRedbookMediaTypeVideo:
		ret.VideoURLs = rawResp.Data.MediaDownloadURLs
	}

	return ret, nil
}
