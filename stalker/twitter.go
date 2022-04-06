package stalker

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/xIceArcher/go-leah/cache"
	"github.com/xIceArcher/go-leah/config"
	"github.com/xIceArcher/go-leah/consts"
	"github.com/xIceArcher/go-leah/discord"
	"github.com/xIceArcher/go-leah/twitter"
	"github.com/xIceArcher/go-leah/utils"
	"go.uber.org/zap"
)

const (
	CacheKeyTweetStalkerPrefix = "go-leah/tweetStalker/"
	CacheKeyTweetStalkerFormat = CacheKeyTweetStalkerPrefix + "%s/%s"
)

var (
	ErrUserNotStalked = fmt.Errorf("user not stalked")
)

type TweetStalkManager struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	stalker twitter.Stalker
	api     twitter.API

	session *discord.Session
	logger  *zap.SugaredLogger

	cache cache.Cache

	userToChannels   map[string]map[string]int
	channelsToEmbeds map[string]map[string]discord.UpdatableMessageEmbeds
}

func NewTweetStalkManager(cfg *config.Config, session *discord.Session, logger *zap.SugaredLogger) *TweetStalkManager {
	cache, err := cache.NewRedisCache(cfg.Redis)
	if err != nil {
		logger.With(zap.Error(err)).Error("Failed to initialize cache, stalks will only be saved in-memory")
	}

	api := twitter.NewCachedAPI(cfg.Twitter, cache, logger)

	streamStalker := twitter.NewStreamStalker(cfg.Twitter, api, logger)
	pollStalker := twitter.NewPollStalker(cfg.Twitter, api, logger)

	return &TweetStalkManager{
		stalker: twitter.NewCombinedStalker(cfg.Twitter, []twitter.Stalker{streamStalker, pollStalker}, logger),
		api:     api,
		session: session,
		logger:  logger,
		cache:   cache,

		userToChannels:   make(map[string]map[string]int),
		channelsToEmbeds: make(map[string]map[string]discord.UpdatableMessageEmbeds),
	}
}

func (t *TweetStalkManager) Start() error {
	t.ctx, t.cancel = context.WithCancel(context.Background())

	if t.cache != nil {
		if err := t.stalker.AddUsers(t.restoreUserIDsFromCache()...); err != nil {
			return err
		}
	}

	if err := t.stalker.Start(); err != nil {
		return err
	}

	go t.handleTweetsTask(t.ctx, t.stalker.OutCh())
	return nil
}

func (t *TweetStalkManager) Stalk(channelID string, screenName string, d time.Duration) error {
	screenName = strings.TrimPrefix(screenName, "@")
	logger := t.logger.With(zap.String("channelID", channelID)).With(zap.String("screenName", screenName))

	user, err := t.api.GetUserByScreenName(screenName)
	if err != nil {
		return err
	}

	if err := t.stalker.AddUsers(user.ID); err != nil {
		return err
	}

	if _, ok := t.userToChannels[user.ID]; !ok {
		t.userToChannels[user.ID] = make(map[string]int)
	}

	colorInt := utils.ParseHexColor(consts.ColorNone)
	t.userToChannels[user.ID][channelID] = colorInt
	if err := t.cache.SetWithExpiry(t.ctx, getCacheKey(channelID, user.ID), colorInt, d); err != nil {
		logger.With(zap.Error(err)).Error("Failed to set cache")
	}

	if d > 0 {
		go t.autoUnstalkTask(t.ctx, channelID, screenName, d)
	}

	return nil
}

func (t *TweetStalkManager) Unstalk(channelID string, screenName string) error {
	screenName = strings.TrimPrefix(screenName, "@")
	logger := t.logger.With(zap.String("channelID", channelID)).With(zap.String("screenName", screenName))

	user, err := t.api.GetUserByScreenName(screenName)
	if err != nil {
		return err
	}

	if channels, ok := t.userToChannels[user.ID]; ok {
		delete(channels, channelID)
		if len(channels) == 0 {
			delete(t.userToChannels, user.ID)
			t.stalker.RemoveUsers(user.ID)
		}
	}

	if err := t.cache.Clear(t.ctx, getCacheKey(channelID, user.ID)); err != nil {
		logger.With(zap.Error(err)).Error("Failed to clear cache")
	}

	return nil
}

func (t *TweetStalkManager) Stalks(channelID string) ([]string, error) {
	userIDs := make([]string, 0)
	for userID, channels := range t.userToChannels {
		if _, ok := channels[channelID]; ok {
			userIDs = append(userIDs, userID)
		}
	}

	screenNames := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		user, err := t.api.GetUser(userID)
		if err != nil {
			return nil, err
		}
		screenNames = append(screenNames, fmt.Sprintf("@%s", user.ScreenName))
	}

	return screenNames, nil
}

func (t *TweetStalkManager) Color(channelID string, screenName string, color int) error {
	user, err := t.api.GetUserByScreenName(screenName)
	if err != nil {
		return err
	}

	channels, ok := t.userToChannels[user.ID]
	if !ok {
		return ErrUserNotStalked
	}

	if _, ok := channels[channelID]; !ok {
		return ErrUserNotStalked
	}

	channels[channelID] = color
	return t.cache.SetKeepTTL(t.ctx, getCacheKey(channelID, user.ID), color)
}

func (t *TweetStalkManager) Stop() {
	t.cancel()
	t.stalker.Stop()

	t.wg.Wait()
}

func (t *TweetStalkManager) restoreUserIDsFromCache() []string {
	keys, err := t.cache.GetByPrefixWithTTL(t.ctx, CacheKeyTweetStalkerPrefix)
	if err != nil {
		t.logger.With(zap.Error(err)).Error("Failed to read keys from cache")
		return nil
	}

	userIDs := make([]string, 0, len(keys))
	for key, val := range keys {
		key = strings.TrimPrefix(key, CacheKeyTweetStalkerPrefix)
		keySplit := strings.Split(key, "/")
		if len(keySplit) != 2 {
			t.logger.With(zap.String("key", key)).Warn("Unknown key")
			continue
		}

		channelID, userID := keySplit[0], keySplit[1]

		if _, ok := t.userToChannels[userID]; !ok {
			t.userToChannels[userID] = make(map[string]int)
		}

		colorStr, ok := val.Value.(string)
		if !ok {
			t.logger.With(zap.String("colorStr", colorStr)).Warn("Cannot parse color")
			colorStr = "000000"
		}

		colorInt, err := strconv.ParseInt(colorStr, 10, 0)
		if err != nil {
			t.logger.With(zap.String("colorStr", colorStr)).With(zap.Error(err)).Warn("Cannot parse color")
			colorInt = 0
		}

		t.userToChannels[userID][channelID] = int(colorInt)

		if val.TTL > 0 {
			user, err := t.api.GetUser(userID)
			if err != nil {
				t.logger.With(zap.Error(err)).Error("Failed to find user")
				continue
			}

			go t.autoUnstalkTask(t.ctx, channelID, user.ScreenName, val.TTL)
		}

		userIDs = append(userIDs, userID)
	}

	return userIDs
}

func (t *TweetStalkManager) autoUnstalkTask(ctx context.Context, channelID string, screenName string, d time.Duration) {
	t.wg.Add(1)
	defer t.wg.Done()

	logger := t.logger.With(zap.String("channelID", channelID)).With(zap.String("screenName", screenName))

	select {
	case <-ctx.Done():
		return
	case <-time.After(d):
		logger.Info("Auto-unstalking...")
		if err := t.Unstalk(channelID, screenName); err != nil {
			logger.With(zap.Error(err)).Error("Failed to unstalk")
		}

		t.session.SendMessage(channelID, fmt.Sprintf("Unstalked @%s in this channel!", screenName))
	}
}

func (t *TweetStalkManager) handleTweetsTask(ctx context.Context, ch <-chan *twitter.Tweet) {
	t.wg.Add(1)
	defer t.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case tweet := <-ch:
			if channelIDs, ok := t.userToChannels[tweet.User.ID]; ok {
				logger := t.logger.With(zap.String("tweetURL", tweet.URL()))
				embeds := tweet.GetEmbeds()

				for channelID, color := range channelIDs {
					logger := logger.With(zap.String("channelID", channelID))

					if _, ok := t.channelsToEmbeds[channelID]; !ok {
						t.channelsToEmbeds[channelID] = make(map[string]discord.UpdatableMessageEmbeds)
					}

					if !t.isTweetRelevant(tweet, channelID) {
						continue
					}

					if tweet.IsRetweet {
						if existingEmbeds, ok := t.channelsToEmbeds[channelID][tweet.RetweetedStatus.ID]; ok {
							t.handleRetweet(existingEmbeds, tweet, logger)
							continue
						}
					}

					for _, embed := range embeds {
						embed.Color = color
					}

					updatableEmbeds, err := t.session.SendEmbeds(channelID, embeds)
					if err != nil {
						logger.With(zap.Error(err)).Error("Failed to send embed")
						continue
					}

					if tweet.HasVideo {
						t.session.SendVideo(channelID, tweet.VideoURL, tweet.ID)
					}

					t.channelsToEmbeds[channelID][tweet.GetBaseID()] = updatableEmbeds
					logger.Info("Sent tweet")
				}
			}
		}
	}
}

func (t *TweetStalkManager) handleRetweet(embeds discord.UpdatableMessageEmbeds, tweet *twitter.Tweet, logger *zap.SugaredLogger) {
	originalTimeStr := embeds[len(embeds)-1].Timestamp
	originalTime, ok := utils.ParseISOTime(originalTimeStr)
	if !ok {
		logger.Errorf("Failed to parse timestamp %s", originalTimeStr)
	}
	d := tweet.Timestamp.Sub(originalTime)

	retweetInfoStr := fmt.Sprintf("\n%s (%s later)", discord.GetNamedLink(tweet.User.Name, tweet.User.URL()), utils.FormatDuration(d))

	found := false
	for _, field := range embeds[0].Fields {
		if field.Name == "Retweeted by" {
			found = true
			field.Value += retweetInfoStr
		}
	}

	if !found {
		embeds[0].Fields = append(embeds[0].Fields, &discordgo.MessageEmbedField{
			Name:  "Retweeted by",
			Value: retweetInfoStr,
		})
	}

	if err := embeds.Update(); err != nil {
		logger.With(zap.Error(err)).Error("Failed to update embed")
	}

	logger.Info("Sent retweet")
}

func (t *TweetStalkManager) isTweetRelevant(tweet *twitter.Tweet, channelID string) bool {
	if !tweet.IsReply {
		return true
	}

	replyUserChannels, ok := t.userToChannels[tweet.ReplyUser.ID]
	if !ok {
		return false
	}

	if _, ok := replyUserChannels[channelID]; !ok {
		return false
	}

	return true
}

func getCacheKey(channelID string, userID string) string {
	return fmt.Sprintf(CacheKeyTweetStalkerFormat, channelID, userID)
}
