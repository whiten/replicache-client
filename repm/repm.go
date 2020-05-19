// +build !js,!wasm

// Package repm implements an Android and iOS interface to Replicache via [Gomobile](https://github.com/golang/go/wiki/Mobile).
// repm is not thread-safe. Callers must guarantee that it is not called concurrently on different threads/goroutines.
package repm

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/attic-labs/noms/go/spec"
	zl "github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	"roci.dev/diff-server/util/log"

	// Log all http request/response pairs.
	_ "roci.dev/diff-server/util/loghttp"

	"roci.dev/replicache-client/api"
)

var (
	repDir string
)

// Logger allows client to optionally provide a place to send repm's log messages.
type Logger interface {
	io.Writer
}

// Init initializes Replicache. If the specified storage directory doesn't exist, it
// is created. Logger receives logging output from Replicache.
func Init(storageDir, tempDir string, logger Logger) {
	if logger == nil {
		zlog.Logger = zlog.Output(zl.ConsoleWriter{Out: os.Stdout})
	} else {
		zlog.Logger = zlog.Output(zl.ConsoleWriter{Out: logger, NoColor: true})
	}

	zl.SetGlobalLevel(zl.InfoLevel)
	l := log.Default()

	if storageDir == "" {
		l.Error().Msg("storageDir must be non-empty")
		return
	}
	if tempDir != "" {
		os.Setenv("TMPDIR", tempDir)
	}

	repDir = storageDir
}

func init() {
	api.Register(list, resolve, drop)
}

// for testing
func deinit() {
	repDir = ""
	api.Reset()
}

func Dispatch(dbName, rpc string, data []byte) (ret []byte, err error) {
	return api.Dispatch(dbName, rpc, data)
}

type DatabaseInfo struct {
	Name string `json:"name"`
}

type ListResponse struct {
	Databases []DatabaseInfo `json:"databases"`
}

func list(l zl.Logger) (resBytes []byte, err error) {
	if repDir == "" {
		return nil, errors.New("must call init first")
	}

	resp := ListResponse{
		Databases: []DatabaseInfo{},
	}

	fi, err := os.Stat(repDir)
	if err != nil {
		if os.IsNotExist(err) {
			return json.Marshal(resp)
		}
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errors.New("Specified path is not a directory")
	}
	entries, err := ioutil.ReadDir(repDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		fmt.Println("In list, got", entry.Name(), entry.IsDir())
		if entry.IsDir() {
			b, err := base64.RawURLEncoding.DecodeString(entry.Name())
			if err != nil {
				l.Err(err).Msgf("Could not decode directory name: %s, skipping", entry.Name())
				continue
			}
			resp.Databases = append(resp.Databases, DatabaseInfo{
				Name: string(b),
			})
		}
	}
	return json.Marshal(resp)
}

func resolve(dbName string, l zl.Logger) (spec.Spec, error) {
	if repDir == "" {
		return spec.Spec{}, errors.New("Replicache is uninitialized - must call init first")
	}
	p := dbPath(repDir, dbName)
	return spec.ForDatabase(p)
}

// Drop closes and deletes the specified local database. Remote replicas in the group are not affected.
func drop(dbName string) error {
	if repDir == "" {
		return errors.New("Replicache is uninitialized - must call init first")
	}
	if dbName == "" {
		return errors.New("dbName must be non-empty")
	}

	db := api.Get(dbName)
	p := dbPath(repDir, dbName)
	if db != nil {
		/*
			NOCOMMIT
			if conn.dir != p {
				return fmt.Errorf("open database %s has directory %s, which is different than specified %s",
				dbName, conn.dir, p)
			}
		*/
		api.Close(dbName)
	}
	return os.RemoveAll(p)
}

func dbPath(root, name string) string {
	return path.Join(root, base64.RawURLEncoding.EncodeToString([]byte(name)))
}
