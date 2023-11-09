package tiktok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/anaskhan96/soup"
	"github.com/xIceArcher/go-leah/config"
)

type API struct{}

func NewAPI(cfg *config.TiktokConfig) (*API, error) {
	return &API{}, nil
}

func (a *API) GetVideo(postID string) (*Video, error) {
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

		if format.VideoCodec == "h265" {
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

	author, err := a.GetUser(rawResp.Uploader)
	if err != nil {
		author = &User{
			ID:       rawResp.UploaderID,
			UniqueID: rawResp.Uploader,
			Nickname: rawResp.Creator,
		}
	}

	return &Video{
		ID:          rawResp.ID,
		Description: rawResp.Description,
		Video:       vidResp.Body,
		Music: &Music{
			AuthorName: rawResp.Artist,
			Title:      rawResp.Track,
		},
		Author:       author,
		LikeCount:    rawResp.LikeCount,
		CommentCount: rawResp.CommentCount,
		ShareCount:   rawResp.RepostCount,
		CreateTime:   time.Unix(rawResp.Timestamp, 0),
	}, nil
}

func (API) GetUser(userID string) (*User, error) {
	resp, err := soup.Get(fmt.Sprintf("https://www.tiktok.com/@%s", userID))
	if err != nil {
		return nil, err
	}

	element := soup.HTMLParse(resp).Find("script", "id", "__UNIVERSAL_DATA_FOR_REHYDRATION__")
	if element.Pointer == nil {
		return nil, fmt.Errorf("could not find element")
	}
	if element.Pointer.FirstChild == nil {
		return nil, fmt.Errorf("could not find child of element")
	}

	rawUserResp := &RawUser{}
	if err := json.Unmarshal([]byte(element.Pointer.FirstChild.Data), rawUserResp); err != nil {
		return nil, err
	}
	rawUser := rawUserResp.DefaultScope.WebappUserDetail.UserInfo.User

	return &User{
		ID:        rawUser.ID,
		UniqueID:  rawUser.UniqueID,
		Nickname:  rawUser.Nickname,
		AvatarURL: rawUser.AvatarLarger,
	}, nil
}
