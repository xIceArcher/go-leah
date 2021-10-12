package qnap

import "fmt"

const (
	StatusStartChunkedUploadOK Status = 0
	StatusOK                   Status = 1
	StatusDisabled             Status = 8
)

var (
	ErrNotLoggedIn error = fmt.Errorf("not logged in")
	ErrFailed      error = fmt.Errorf("failed")
)
