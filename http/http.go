package http

import (
	"io"
	"time"
)

const (
	StatusOK           = 200
	StatusUnauthorized = 401
)

type Response struct {
	Status     string
	StatusCode int
	Body       io.Reader
}

func Get(url string, timeout time.Duration) (*Response, error) {
	return impl("GET", url, nil, timeout, "")
}

func Post(url string, body []byte, timeout time.Duration, auth string) (*Response, error) {
	return impl("POST", url, body, timeout, auth)
}
