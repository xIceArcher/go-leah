package twitter

import "go.uber.org/zap"

type Stalker interface {
	AddUsers(userIDs ...string) error
	RemoveUsers(userIDs ...string) error
	IsStalkingUser(userID string) bool

	OutCh() <-chan *Tweet
}

type twitterStalker struct {
	userIDs map[string]struct{}
	outCh   chan *Tweet

	restartCh    chan int
	restartErrCh chan error

	logger *zap.SugaredLogger
}

func newTwitterStalker(logger *zap.SugaredLogger) *twitterStalker {
	return &twitterStalker{
		userIDs: make(map[string]struct{}),
		outCh:   make(chan *Tweet),

		restartCh:    make(chan int),
		restartErrCh: make(chan error),

		logger: logger,
	}
}

func (s *twitterStalker) AddUsers(userIDs ...string) error {
	shouldRestart := false
	for _, userID := range userIDs {
		if s.IsStalkingUser(userID) {
			continue
		}

		shouldRestart = true
		s.userIDs[userID] = struct{}{}
	}

	if shouldRestart {
		s.restartCh <- 1
		return <-s.restartErrCh
	}

	return nil
}

func (s *twitterStalker) RemoveUsers(userIDs ...string) error {
	shouldRestart := false
	for _, userID := range userIDs {
		if !s.IsStalkingUser(userID) {
			continue
		}

		shouldRestart = true
		delete(s.userIDs, userID)
	}

	if shouldRestart {
		s.restartCh <- 1
		return <-s.restartErrCh
	}

	return nil
}

func (s *twitterStalker) IsStalkingUser(userID string) bool {
	_, ok := s.userIDs[userID]
	return ok
}

func (s *twitterStalker) OutCh() <-chan *Tweet {
	return s.outCh
}
