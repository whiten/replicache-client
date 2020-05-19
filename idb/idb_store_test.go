// +build js,wasm

package idb

import (
	"testing"

	"github.com/lithammer/shortuuid"
	"github.com/stretchr/testify/suite"

	"github.com/attic-labs/noms/go/chunks"
	"github.com/attic-labs/noms/go/chunks/chunkstest"
)

type indexedDBStoreFactory struct {
	prefix string
}

func NewIndexedDBStoreFactory(prefix string) chunks.Factory {
	return &indexedDBStoreFactory{prefix}
}

func (f *indexedDBStoreFactory) CreateStoreFromCache(ns string) chunks.ChunkStore {
	return f.CreateStore(ns)
}

func (f *indexedDBStoreFactory) CreateStore(ns string) chunks.ChunkStore {
	ns = f.prefix + ns

	store, err := NewIndexedDBStore(ns)
	if err != nil {
		panic("NewIndexedDBStore failed: " + err.Error())
	}
	return store
}

func (f *indexedDBStoreFactory) Shutter() {
}

func TestIndexedDBStoreTestSuite(t *testing.T) {
	suite.Run(t, &IndexedDBStoreTestSuite{})
}

type IndexedDBStoreTestSuite struct {
	chunkstest.ChunkStoreTestSuite
}

func (suite *IndexedDBStoreTestSuite) SetupTest() {
	suite.Factory = NewIndexedDBStoreFactory(shortuuid.New() + "-")
}

func (suite *IndexedDBStoreTestSuite) TearDownTest() {
	suite.Factory.Shutter()
}
