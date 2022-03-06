package tiktok

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

type API struct{}

var (
	tiktokVideotURLFormat string
	client                *http.Client
	apiSetupOnce          sync.Once
)

func NewAPI(cfg *config.TiktokConfig) (*API, error) {
	apiSetupOnce.Do(func() {
		tiktokVideotURLFormat = cfg.VideoURLFormat
		client = &http.Client{
			Timeout: 10 * time.Second,
		}
	})

	return &API{}, nil
}

func (API) GetVideo(postID string) (*Video, error) {
	resp, err := client.Get(fmt.Sprintf(tiktokVideotURLFormat, postID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %v", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	rawResp := TiktokResp{}
	if err := json.Unmarshal(body, &rawResp); err != nil {
		return nil, err
	}

	rawVideo, ok := rawResp.RawItem[postID]
	if !ok {
		return nil, fmt.Errorf("failed to get video")
	}

	rawAuthor, ok := rawResp.RawUser.Users[rawVideo.AuthorUniqueID]
	if !ok {
		return nil, fmt.Errorf("failed to get user")
	}

	createTimeInt, err := strconv.ParseInt(rawVideo.CreateTime, 10, 64)
	if err != nil {
		return nil, err
	}

	video := &Video{
		ID:          postID,
		Description: rawVideo.Description,
		VideoURL:    rawVideo.Video.DownloadAddr,

		Music: &Music{
			ID:         rawVideo.Music.ID,
			Album:      rawVideo.Music.Album,
			AuthorName: rawVideo.Music.AuthorName,
			Title:      rawVideo.Music.Title,
		},

		Author: &User{
			ID:        rawAuthor.ID,
			UniqueID:  rawAuthor.UniqueID,
			Nickname:  rawAuthor.Nickname,
			AvatarURL: rawAuthor.AvatarLarger,
		},

		LikeCount:    rawVideo.Stats.DiggCount,
		CommentCount: rawVideo.Stats.CommentCount,
		ShareCount:   rawVideo.Stats.ShareCount,

		CreateTime: time.Unix(createTimeInt, 0),
	}

	for _, tag := range rawVideo.TextExtra {
		if tag.HashtagName != "" {
			hashtagText := utils.SliceUTF16String(rawVideo.Description, tag.Start, tag.End)

			if !strings.HasSuffix(strings.ToLower(hashtagText), tag.HashtagName) {
				zap.S().With(zap.String("expected", tag.HashtagName)).With("actual", hashtagText).Warn("Inconsistent hashtag text")
				continue
			}

			video.Tags = append(video.Tags, utils.NewEntity(
				utils.GetUTF16StringIdx(rawVideo.Description, tag.Start),
				hashtagText,
			))
		} else if tag.UserUniqueID != "" {
			mentionText := utils.SliceUTF16String(rawVideo.Description, tag.Start, tag.End)

			if !strings.HasSuffix(strings.ToLower(mentionText), tag.UserUniqueID) {
				zap.S().With(zap.String("expected", tag.UserUniqueID)).With("actual", mentionText).Warn("Inconsistent mention text")
				continue
			}

			video.Mentions = append(video.Mentions, utils.NewEntity(
				utils.GetUTF16StringIdx(rawVideo.Description, tag.Start),
				mentionText,
			))
		}
	}

	return video, nil
}

func GetTagURL(s string) string {
	firstChar := string([]rune(s)[0:1])
	if firstChar == "#" || firstChar == "＃" {
		s = string([]rune(s)[1:])
	}

	return fmt.Sprintf("https://www.tiktok.com/tag/%s", s)
}

func GetMentionURL(s string) string {
	firstChar := string([]rune(s)[0:1])
	if firstChar == "@" || firstChar == "＠" {
		s = string([]rune(s)[1:])
	}

	return fmt.Sprintf("https://www.tiktok.com/@%s", s)
}
