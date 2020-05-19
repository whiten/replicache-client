package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/attic-labs/noms/go/spec/lite"
	zl "github.com/rs/zerolog"

	"roci.dev/diff-server/util/chk"
	"roci.dev/diff-server/util/log"
	"roci.dev/diff-server/util/time"
	"roci.dev/diff-server/util/version"

	"roci.dev/replicache-client/db"
)

type (
	ListFn    = func(zl.Logger) ([]byte, error)
	ResolveFn = func(string, zl.Logger) (spec.Spec, error)
	DropFn    = func(string) error
)

var (
	list    ListFn    = nil
	resolve ResolveFn = nil
	drop    DropFn    = nil

	connections = map[string]*connection{}

	// Unique rpc request ID
	rid uint64
)

func Register(listFn ListFn, resolveFn ResolveFn, dropFn DropFn) {
	list = listFn
	resolve = resolveFn
	drop = dropFn
}

func Get(dbName string) *db.DB {
	if conn := connections[dbName]; conn != nil {
		return conn.db
	}
	return nil
}

// For testing only.
func Reset() {
	connections = map[string]*connection{}
}

// Dispatch send an API request to Replicache, JSON-serialized parameters, and returns the response.
func Dispatch(dbName, rpc string, data []byte) (ret []byte, err error) {
	t0 := time.Now()
	l := log.Default().With().
		Str("db", dbName).
		Str("req", rpc).
		Uint64("rid", atomic.AddUint64(&rid, 1)).
		Logger()

	l.Debug().Bytes("data", data).Msg("rpc -->")

	defer func() {
		t1 := time.Now()
		l.Debug().Bytes("ret", ret).Dur("dur", t1.Sub(t0)).Msg("rpc <--")
		if r := recover(); r != nil {
			var msg string
			if e, ok := r.(error); ok {
				msg = e.Error()
			} else {
				msg = fmt.Sprintf("%v", r)
			}
			l.Error().Stack().Msgf("Replicache panicked with: %s\n", msg)
			ret = nil
			err = fmt.Errorf("Replicache panicked with: %s - see stderr for more", msg)
		}
	}()

	switch rpc {
	case "list":
		return list(l)
	case "open":
		return nil, open(dbName, l)
	case "close":
		return nil, Close(dbName)
	case "drop":
		return nil, drop(dbName)
	case "version":
		return []byte(version.Version()), nil
	case "profile":
		profile(l)
		return nil, nil
	case "setLogLevel":
		// dbName param is ignored
		return nil, setLogLevel(data)
	}

	conn := connections[dbName]
	if conn == nil {
		return nil, errors.New("specified database is not open")
	}

	l = l.With().Str("cid", conn.db.ClientID()).Logger()

	switch rpc {
	case "getRoot":
		return conn.dispatchGetRoot(data)
	case "has":
		return conn.dispatchHas(data)
	case "get":
		return conn.dispatchGet(data)
	case "scan":
		return conn.dispatchScan(data)
	case "put":
		return conn.dispatchPut(data)
	case "del":
		return conn.dispatchDel(data)
	case "beginSync":
		return conn.dispatchBeginSync(data, l)
	case "maybeEndSync":
		return conn.dispatchMaybeEndSync(data)
	case "openTransaction":
		return conn.dispatchOpenTransaction(data)
	case "closeTransaction":
		return conn.dispatchCloseTransaction(data)
	case "commitTransaction":
		return conn.dispatchCommitTransaction(data, l)
	}
	chk.Fail("Unsupported rpc name: %s", rpc)
	return nil, nil
}

// Open a Replicache database. If the named database doesn't exist it is created.
func open(dbName string, l zl.Logger) error {
	if dbName == "" {
		return errors.New("dbName must be non-empty")
	}

	if _, ok := connections[dbName]; ok {
		return nil
	}

	sp, err := resolve(dbName, l)
	if err != nil {
		return err
	}
	db, err := db.Load(sp)
	if err != nil {
		return err
	}

	l.Info().Msgf("Opened Replicache instance at: %s with tempdir: %s and ClientID: %s", sp.String(), os.TempDir(), db.ClientID())
	connections[dbName] = newConnection(db)
	return nil
}

// Close releases the resources held by the specified open database.
func Close(dbName string) error {
	if dbName == "" {
		return errors.New("dbName must be non-empty")
	}
	conn := connections[dbName]
	if conn == nil {
		return nil
	}
	delete(connections, dbName)
	return nil
}

func setLogLevel(data []byte) error {
	var ls string
	err := json.Unmarshal(data, &ls)
	if err != nil {
		return err
	}

	err = log.SetGlobalLevelFromString(ls)
	if err != nil {
		return err
	}

	return nil
}
