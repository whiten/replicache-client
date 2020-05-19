// +build !js,!wasm

package db

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/attic-labs/noms/go/spec/lite"
)

func TestLoadBadSpec(t *testing.T) {
	assert := assert.New(t)

	sp, err := spec.ForDatabase("http://localhost:6666") // not running, presumably
	assert.NoError(err)
	db, err := Load(sp)
	assert.Nil(db)
	assert.Regexp(`Get "?http://localhost:6666/root/"?: dial tcp (.+?):6666: connect: connection refused`, err.Error())

	srv := httptest.NewServer(http.NotFoundHandler())
	sp, err = spec.ForDatabase(srv.URL)
	assert.NoError(err)
	db, err = Load(sp)
	assert.Nil(db)
	assert.EqualError(err, "Unexpected response: Not Found: 404 page not found")
}
