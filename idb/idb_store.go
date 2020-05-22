// +build js,wasm

package idb

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/attic-labs/noms/go/chunks"
	"github.com/attic-labs/noms/go/constants"
	"github.com/attic-labs/noms/go/datas"
	"github.com/attic-labs/noms/go/hash"
	"github.com/attic-labs/noms/go/spec/lite"
)

const (
	rootKey = "root"
)

type IndexedDB interface {
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)

	OpenTransaction() (Transaction, error)
}

type Transaction interface {
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
	Put(key, value []byte) error

	Commit() error
}

type IndexedDBStore struct {
	idb     IndexedDB
	pending map[hash.Hash]chunks.Chunk
	mu      sync.RWMutex
	root    hash.Hash
}

func NewIndexedDBStore(name string) (*IndexedDBStore, error) {
	db, err := NewIndexedDB(name)
	if err != nil {
		return nil, err
	}
	return &IndexedDBStore{idb: db, pending: make(map[hash.Hash]chunks.Chunk)}, nil
}

func (s *IndexedDBStore) Get(h hash.Hash) chunks.Chunk {
	{
		s.mu.RLock()
		defer s.mu.RUnlock()
		if c, ok := s.pending[h]; ok {
			return c
		}
	}

	value, err := s.idb.Get([]byte(h.String()))
	if err != nil {
		return chunks.Chunk{}
	}
	ch := make(chan *chunks.Chunk, 10)
	err = chunks.Deserialize(bytes.NewBuffer(value), ch)
	close(ch)
	var res chunks.Chunk
	for c := range ch {
		if c.Hash() == h {
			res = *c
			break
		}
	}

	return res
}

func (s *IndexedDBStore) GetMany(hashes hash.HashSet, foundChunks chan *chunks.Chunk) {
	fmt.Println("NOCOMMIT GetMany")
}

func (s *IndexedDBStore) Has(h hash.Hash) bool {
	{
		s.mu.RLock()
		defer s.mu.RUnlock()
		if _, ok := s.pending[h]; ok {
			return true
		}
	}
	has, err := s.idb.Has([]byte(h.String()))
	return err == nil && has
}

func (s *IndexedDBStore) HasMany(hashes hash.HashSet) (absent hash.HashSet) {
	absent = hash.HashSet{}
	for h := range hashes {
		if !s.Has(h) {
			absent.Insert(h)
		}
	}
	return absent
}

func (s *IndexedDBStore) Put(c chunks.Chunk) {
	s.pending[c.Hash()] = c
}

func (s *IndexedDBStore) Version() string {
	return constants.NomsVersion
}

func (s *IndexedDBStore) Rebase() {
	// NOCOMMIT
	fmt.Println("NOCOMMIT Rebase")
}

func (s *IndexedDBStore) Root() hash.Hash {
	if s.root.IsEmpty() {
		v, err := s.idb.Get([]byte(rootKey))
		if err != nil {
			fmt.Println("Read root failed")
		} else if root, ok := hash.MaybeParse(string(v)); ok {
			s.root = root
		}
	}
	return s.root
}

func (s *IndexedDBStore) Commit(current, last hash.Hash) bool {
	// NOCOMMIT: How to rollback transaction if an operation fails?
	if last != s.root {
		return false
	}
	t, err := s.idb.OpenTransaction()
	if err != nil {
		return false
	}

	v, err := t.Get([]byte(rootKey))
	if err != nil {
		fmt.Println("Transaction get failed")
		return false
	}
	if string(v) != last.String() {
		if len(v) != 0 || last.String() != "00000000000000000000000000000000" {
			fmt.Println("Transaction read failed", len(v), string(v), last.String())
			panic("Transaction read failed")
			return false
		}
	}

	ch := make(chan error)
	for key, c := range s.pending {
		buf := bytes.Buffer{}
		buf.Reset()
		chunks.Serialize(c, &buf)
		go func(key string) {
			ch <- t.Put([]byte(key), buf.Bytes())
		}(key.String())
	}

	go func() {
		ch <- t.Put([]byte(rootKey), []byte(current.String()))
	}()

	for i := 0; i < len(s.pending)+1; i++ {
		if err = <-ch; err != nil {
			fmt.Println("Put failed", err)
			return false
		}
	}
	err = t.Commit()
	if err != nil {
		fmt.Println("Commit failed!", err)
		return false
	}
	s.root = current

	s.pending = make(map[hash.Hash]chunks.Chunk)
	return true
}

func (s *IndexedDBStore) Stats() interface{} {
	return nil
}

func (s *IndexedDBStore) StatsSummary() string {
	return "Unsupported"
}

func (s *IndexedDBStore) Close() error {
	fmt.Println("NOCOMMIT Close")
	return nil
}

type indexedDBProtocol struct{}

func (p indexedDBProtocol) NewChunkStore(sp spec.Spec) (chunks.ChunkStore, error) {
	return NewIndexedDBStore(sp.DatabaseName)
}

func (p indexedDBProtocol) NewDatabase(sp spec.Spec) (datas.Database, error) {
	return datas.NewDatabase(sp.NewChunkStore()), nil
}

func init() {
	spec.ExternalProtocols["idb"] = &indexedDBProtocol{}
}
