package utils

import (
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
