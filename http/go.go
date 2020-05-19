// +build !js,!wasm

package http

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

func impl(method, url string, body []byte, timeout time.Duration, auth string) (*Response, error) {
	var reader io.Reader
	if len(body) != 0 {
		reader = bytes.NewReader(body)
	}
	httpReq, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Add("Authorization", auth)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Body:       resp.Body,
	}, nil

}
