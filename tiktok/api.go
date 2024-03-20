package tiktok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/anaskhan96/soup"
	"github.com/xIceArcher/go-leah/config"
	"go.uber.org/zap"
)

type API struct{}

func NewAPI(cfg *config.TiktokConfig) (*API, error) {
	return &API{}, nil
}

func (a *API) GetVideo(postID string) (*Video, error) {
	url := fmt.Sprintf("https://www.tiktok.com/@a/video/%s", postID)

	cmd := exec.Command("yt-dlp", url, "-j")
	out := &bytes.Buffer{}
	cmd.Stdout = out

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	rawResp := &RawVideo{}
	if err := json.NewDecoder(out).Decode(rawResp); err != nil {
		return nil, err
	}

	var fileName string
	for _, format := range rawResp.Formats {
		logger := zap.S().With("format", format.Format)

		if format.IsWatermarked() {
			logger.Info("Watermarked")
			continue
		}

		if format.VideoCodec == "h265" {
			logger.Info("H265 codec")
			continue
		}

		file, err := os.CreateTemp("", fmt.Sprintf("*-%s.mp4", postID))
		if err != nil {
			logger.With(zap.Error(err)).Info("Failed to create temp file")
			return nil, err
		}

		if err := file.Close(); err != nil {
			logger.With(zap.Error(err)).Info("Failed to create temp file")
			return nil, err
		}

		downloadCmd := exec.Command("yt-dlp", url, "-f", format.FormatID, "-o", file.Name(), "--force-overwrites")
		downloadOut := &bytes.Buffer{}
		cmd.Stdout = downloadOut

		if err := downloadCmd.Run(); err != nil {
			logger.With(zap.Error(err)).Info("Failed to run download command")
			continue
		}

		fileName = file.Name()
		break
	}

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
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
		Video:       file,
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
