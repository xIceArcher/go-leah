package utils

import (
	"errors"
	"io"
	"net/http"
	"regexp"
	"time"
)

func ExpandURL(url string, stopRegexes ...*regexp.Regexp) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			for _, regex := range stopRegexes {
				if regex.MatchString(req.URL.String()) {
					return http.ErrUseLastResponse
				}
			}

			return nil
		},
	}

	resp, err := client.Head(url)
	if err != nil {
		return "", err
	}

	return resp.Request.URL.String(), nil
}

var downloadClient = &http.Client{
	Timeout: 10 * time.Second,
}
var ErrResponseTooLong = errors.New("response too long")

func Download(url string, maxBytes int64) (io.Reader, int64, error) {
	resp, err := downloadClient.Head(url)
	if err != nil {
		return nil, 0, err
	}
	if resp.ContentLength > maxBytes {
		return nil, resp.ContentLength, ErrResponseTooLong
	}

	resp, err = downloadClient.Get(url)
	if err != nil {
		return nil, 0, err
	}

	return resp.Body, resp.ContentLength, nil
}
