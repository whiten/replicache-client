package http

import (
	"encoding/json"
	"io/ioutil"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type httpBinResponse struct {
	Headers map[string]string
	Data    string
	Json    map[string]string
	Gzipped,
	Brotli bool
}

func TestBadDomain(t *testing.T) {
	assert := assert.New(t)
	resp, err := Post("https://httpbin.notatld/post", nil, time.Second, "")
	assert.Nil(resp)
	assert.NotNil(err)
	if runtime.GOOS == "js" {
		assert.Contains(err.Error(), "TypeError: Failed to fetch")
	} else {
		assert.Contains(err.Error(), "no such host")
	}
}

func TestBadPath(t *testing.T) {
	assert := assert.New(t)
	resp, err := Post("https://httpbin.org/notapath", nil, time.Second, "")
	assert.Nil(err)
	assert.Equal(resp.StatusCode, 404)
	if runtime.GOOS != "js" {
		assert.Equal(resp.Status, "404 Not Found")
	}
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	assert.Contains(
		string(body), "The requested URL was not found on the server.")
}

func TestCORS(t *testing.T) {
	assert := assert.New(t)

	resp, err := Post("https://postb.in/api/bin", nil, time.Second, "")
	if runtime.GOOS == "js" {
		assert.Nil(resp)
		assert.NotNil(err)
		assert.Contains(err.Error(), "TypeError: Failed to fetch")
	} else {
		assert.NotNil(resp)
		assert.Equal(201, resp.StatusCode)
		assert.NoError(err)
	}
}

func TestPostEmpty(t *testing.T) {
	assert := assert.New(t)
	resp, err := Post("https://httpbin.org/post", nil, time.Second, "auth")
	assert.Nil(err)
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	post := &httpBinResponse{}
	assert.Nil(json.Unmarshal(body, &post))
	assert.Equal("0", post.Headers["Content-Length"])
	assert.Equal("", post.Data)
	assert.Equal("auth", post.Headers["Authorization"])
	if runtime.GOOS == "js" {
		assert.Contains(post.Headers["User-Agent"], "Mozilla/5.0")
	} else {
		assert.Equal(post.Headers["User-Agent"], "Go-http-client/2.0")
	}
}

func TestPost(t *testing.T) {
	assert := assert.New(t)
	data := `{"key": "value"}`
	resp, err := Post("https://httpbin.org/post", []byte(data), time.Second, "auth")
	assert.Nil(err)
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	post := &httpBinResponse{}
	assert.Nil(json.Unmarshal(body, &post))
	assert.Equal("16", post.Headers["Content-Length"])
	assert.Equal(data, post.Data)
	assert.Equal("auth", post.Headers["Authorization"])
	if runtime.GOOS == "js" {
		assert.Contains(post.Headers["User-Agent"], "Mozilla/5.0")
	} else {
		assert.Equal(post.Headers["User-Agent"], "Go-http-client/2.0")
	}
}

func TestGet(t *testing.T) {
	assert := assert.New(t)
	resp, err := Get("https://httpbin.org/get", time.Second)
	assert.Nil(err)
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	post := &httpBinResponse{}
	assert.Nil(json.Unmarshal(body, &post))
	assert.False(post.Gzipped)
	assert.False(post.Brotli)
	if runtime.GOOS == "js" {
		assert.Contains(post.Headers["User-Agent"], "Mozilla/5.0")
	} else {
		assert.Equal(post.Headers["User-Agent"], "Go-http-client/2.0")
	}
}

func TestGzip(t *testing.T) {
	assert := assert.New(t)
	resp, err := Get("https://httpbin.org/gzip", time.Second)
	assert.Nil(err)
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	post := &httpBinResponse{}
	assert.Nil(json.Unmarshal(body, &post))
	assert.True(post.Gzipped)
	if runtime.GOOS == "js" {
		assert.Contains(post.Headers["User-Agent"], "Mozilla/5.0")
	} else {
		assert.Equal(post.Headers["User-Agent"], "Go-http-client/2.0")
	}
}

func TestBrotli(t *testing.T) {
	if runtime.GOOS != "js" {
		t.Skip("Brotli not supported by Go")
	}
	assert := assert.New(t)
	resp, err := Get("https://httpbin.org/brotli", time.Second)
	assert.Nil(err)
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(err)
	post := &httpBinResponse{}
	assert.Nil(json.Unmarshal(body, &post))
	assert.True(post.Brotli)
	assert.Contains(post.Headers["User-Agent"], "Mozilla/5.0")
}
