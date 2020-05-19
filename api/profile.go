// +build !js,!wasm

package api

import (
	"net/http"
	"runtime"

	zl "github.com/rs/zerolog"
)

func profile(l zl.Logger) {
	runtime.SetBlockProfileRate(1)
	go func() {
		l.Info().Msgf("Enabling http profiler: %s", http.ListenAndServe("localhost:6060", nil))
	}()
}
