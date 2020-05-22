package db

import (
	"fmt"
	"testing"

	"github.com/attic-labs/noms/go/types"
	"github.com/stretchr/testify/assert"
	"roci.dev/diff-server/util/log"
)

func benchmarkGet(gets int, b *testing.B) {
	assert := assert.New(b)
	db, _, err := LoadTempDB()
	assert.Nil(err)

	n := b.N
	if gets > b.N {
		n = gets
	}
	keys := make([]string, gets, gets)
	for i := 0; i < gets; i++ {
		keys[i] = fmt.Sprintf("foo-%d", i)
	}
	tx := db.NewTransaction()
	for i := 0; i < n; i++ {
		if i%10000 == 9999 {
			_, err := tx.Commit(log.Default())
			assert.NoError(err)
			tx = db.NewTransaction()
		}
		err := tx.Put(fmt.Sprintf("foo-%d", i),
			[]byte(fmt.Sprintf("%f", types.Number(i))))
		assert.NoError(err)
	}
	_, err = tx.Commit(log.Default())
	assert.NoError(err)
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		tx := db.NewTransaction()
		for m := 0; m < gets; m++ {
			_, err := tx.Get(keys[m])
			assert.NoError(err)
		}
		err = tx.Close()
		assert.NoError(err)
	}
}

func BenchmarkGet1(b *testing.B)    { benchmarkGet(1, b) }
func BenchmarkGet32(b *testing.B)   { benchmarkGet(32, b) }
func BenchmarkGet1024(b *testing.B) { benchmarkGet(1024, b) }

func benchmarkPut(puts int, b *testing.B) {
	assert := assert.New(b)
	db, _, err := LoadTempDB()
	assert.Nil(err)

	keys := make([]string, puts, puts)
	for i := 0; i < puts; i++ {
		keys[i] = fmt.Sprintf("foo-%d", i)
	}
	values := make([][]byte, b.N, b.N)
	for n := 0; n < b.N; n++ {
		values[n] = []byte(fmt.Sprintf("%f", types.Number(n)))
	}
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		tx := db.NewTransaction()
		for m := 0; m < puts; m++ {
			err := tx.Put(keys[m], values[n])
			assert.NoError(err)
		}
		_, err := tx.Commit(log.Default())
		assert.NoError(err)
	}
}

func BenchmarkPut0(b *testing.B)    { benchmarkPut(0, b) }
func BenchmarkPut1(b *testing.B)    { benchmarkPut(1, b) }
func BenchmarkPut32(b *testing.B)   { benchmarkPut(32, b) }
func BenchmarkPut1024(b *testing.B) { benchmarkPut(1024, b) }

func Fibonacci(n uint) uint {
	if n <= 1 {
		return n
	}
	return Fibonacci(n-1) + Fibonacci(n-2)
}

func BenchmarkFibonacci(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Fibonacci(20)
	}
}

func BenchmarkMultiply(b *testing.B) {
	for i := 0; i < b.N; i++ {
		n := 2
		for j := 0; j < 10000; j++ {
			n *= 2
		}
	}
}
