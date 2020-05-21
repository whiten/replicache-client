// +build js,wasm

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	spec "github.com/attic-labs/noms/go/spec/lite"

	"roci.dev/replicache-client/db"
)

func TestRepJS(t *testing.T) {
	assert := assert.New(t)

	sp, err := spec.ForDatabase("idb:test")
	assert.NoError(err)

	r, err := db.Load(sp)
	assert.NoError(err)

	tx := r.NewTransaction()
	defer tx.Close()
	items, err := tx.Scan(db.ScanOptions{})
	assert.Equal(0, len(items))
	// NOCOMMIT: More here.
}
