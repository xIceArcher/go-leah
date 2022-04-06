package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-retryablehttp"
)

type HeadersTransport struct {
	client  *retryablehttp.Client
	Headers map[string]string
}

func (t *HeadersTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.Headers {
		req.Header.Add(k, v)
	}
	return cleanhttp.DefaultTransport().RoundTrip(req)
}

func NewClientWithHeaders(headers map[string]string) *retryablehttp.Client {
	client := retryablehttp.NewClient()
	client.HTTPClient.Timeout = time.Minute
	client.HTTPClient.Transport = &HeadersTransport{
		client:  client,
		Headers: headers,
	}
	client.Logger = nil

	return client
}

func NewClientWithHeadersAndCookiesStr(headerStr string, cookiesStr string) *retryablehttp.Client {
	headers := make(map[string]string)
	if headerStr != "" {
		for _, header := range strings.Split(headerStr, ";") {
			headerKeyVal := strings.Split(header, "=")
			if len(headerKeyVal) != 2 {
				continue
			}

			key, val := headerKeyVal[0], headerKeyVal[1]
			headers[key] = val
		}
	}

	if cookiesStr != "" {
		headers["Cookie"] = cookiesStr
	}

	return NewClientWithHeaders(headers)
}
