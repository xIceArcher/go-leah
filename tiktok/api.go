package tiktok

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/xIceArcher/go-leah/config"
	"gopkg.in/vansante/go-ffprobe.v2"
)

type API struct{}

func NewAPI(cfg *config.TiktokConfig) (*API, error) {
	return &API{}, nil
}

func (API) GetVideo(postID string) (*Video, error) {
	cmd := exec.Command("yt-dlp", fmt.Sprintf("https://www.tiktok.com/@a/video/%s", postID), "-j")
	out := &bytes.Buffer{}
	cmd.Stdout = out

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	rawResp := &RawVideo{}
	if err := json.NewDecoder(out).Decode(rawResp); err != nil {
		return nil, err
	}

	var vidResp *http.Response
	for _, format := range rawResp.Formats {
		if format.IsWatermarked() {
			continue
		}

		ffprobeData, err := ffprobe.ProbeURL(context.Background(), format.URL)
		if err != nil {
			continue
		}

		if len(ffprobeData.Streams) == 0 {
			continue
		}

		if ffprobeData.Streams[0].CodecName == "hevc" {
			continue
		}

		req, err := http.NewRequest(http.MethodGet, format.URL, nil)
		if err != nil {
			continue
		}

		for k, v := range format.HTTPHeaders {
			req.Header.Add(k, v)
		}
		if format.Cookies != "" {
			req.Header.Add("Cookie", format.Cookies)
		}

		currVidResp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		if currVidResp.StatusCode == http.StatusOK {
			vidResp = currVidResp
			break
		}
	}

	if vidResp == nil {
		return nil, fmt.Errorf("failed to get videos")
	}

	return &Video{
		ID:          rawResp.ID,
		Description: rawResp.Description,
		Video:       vidResp.Body,
		Music: &Music{
			AuthorName: rawResp.Artist,
			Title:      rawResp.Track,
		},
		Author: &User{
			ID:       rawResp.UploaderID,
			UniqueID: rawResp.Uploader,
			Nickname: rawResp.Creator,
		},
		LikeCount:    rawResp.LikeCount,
		CommentCount: rawResp.CommentCount,
		ShareCount:   rawResp.RepostCount,
		CreateTime:   time.Unix(rawResp.Timestamp, 0),
	}, nil
}
