package tiktok

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"time"

	"go.uber.org/zap"
)

var (
	tikTokAvatarRegexes = []*regexp.Regexp{
		regexp.MustCompile(`"avatarLarger"\s*:\s*"([^"]+)"`),
		regexp.MustCompile(`"avatarMedium"\s*:\s*"([^"]+)"`),
		regexp.MustCompile(`"avatarThumb"\s*:\s*"([^"]+)"`),
	}
)

type API struct{}

func NewAPI() (*API, error) {
	return &API{}, nil
}

func (a *API) GetVideo(postID string) (*Video, error) {
	logger := zap.S().With("postID", postID)

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
	rawResp.Formats.SortByQuality()

	var fileName string
	couldDownloadFormat := false
	for _, format := range rawResp.Formats {
		logger := logger.With("format", format.Format)

		file, err := os.CreateTemp("", fmt.Sprintf("*-%s.mp4", postID))
		if err != nil {
			logger.With(zap.Error(err)).Info("Failed to create temp file")
			continue
		}
		defer os.Remove(file.Name())

		if err := file.Close(); err != nil {
			logger.With(zap.Error(err)).Info("Failed to create temp file")
			continue
		}

		downloadCmd := exec.Command("yt-dlp", url, "-f", format.FormatID, "-o", file.Name(), "--force-overwrites")
		downloadOut := &bytes.Buffer{}
		cmd.Stdout = downloadOut

		if err := downloadCmd.Run(); err != nil {
			logger.With(zap.Error(err)).Info("Failed to run download command")
			continue
		}

		fileName = file.Name()
		couldDownloadFormat = true
		break
	}

	if !couldDownloadFormat {
		return nil, fmt.Errorf("failed to download a single format")
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
	url := fmt.Sprintf("https://www.tiktok.com/@%s", userID)

	cmd := exec.Command("yt-dlp", "-j", "--max-downloads", "1", url)
	out := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = &bytes.Buffer{} // Suppress warnings/stderr

	// yt-dlp may exit with non-zero status due to warnings, but still outputs valid JSON
	cmd.Run()

	var metadata RawUser
	if err := json.NewDecoder(out).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode user metadata: %w", err)
	}

	nickname := metadata.Channel
	if nickname == "" {
		nickname = metadata.Uploader
	}

	// Optional fallback: try to parse the public profile page for avatar URLs.
	// This is best-effort only and does not fail the user lookup if scraping
	// is blocked or if the page structure changes.
	avatarURL := extractTikTokAvatarURL(url)

	return &User{
		ID:        metadata.UploaderID,
		UniqueID:  metadata.Uploader,
		Nickname:  nickname,
		AvatarURL: avatarURL,
	}, nil
}

func extractTikTokAvatarURL(url string) string {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	return findTikTokAvatar(body)
}

func findTikTokAvatar(body []byte) string {
	for _, regex := range tikTokAvatarRegexes {
		match := regex.FindSubmatch(body)
		if len(match) == 2 {
			// The captured URL contains JSON escape sequences like \u002F
			// Properly decode by wrapping in quotes and unmarshaling as JSON
			escapedURL := string(match[1])
			var decodedURL string
			if err := json.Unmarshal([]byte(`"`+escapedURL+`"`), &decodedURL); err == nil && decodedURL != "" {
				return decodedURL
			}
		}
	}
	return ""
}
