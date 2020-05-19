// +build js,wasm

package db

import (
	"github.com/lithammer/shortuuid"

	"github.com/attic-labs/noms/go/spec/lite"

	_ "roci.dev/replicache-client/idb"
)

func LoadTempDB() (r *DB, dir string, err error) {
	dir = "idb:" + shortuuid.New()
	sp, err := spec.ForDatabase(dir)
	if err != nil {
		return
	}

	r, err = Load(sp)
	if err != nil {
		return
	}

	return
}
