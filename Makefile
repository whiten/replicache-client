GO ?= go

.PHONY: js
## Build all Javascript targets
js: build/repjs.gz build/repjs.br build/repjs.debug

.PHONY: test
## Run all tests under the host OS.
test:
	go test ./...

.PHONY: jstest
## Run all tests under js/wasm within a headless Chrome.
jstest:
	GOOS=js GOARCH=wasm go test -exec=wasmbrowsertest ./...

.PHONY: build/repjs
## Build WebAssembly target
build/repjs:
	GOOS=js GOARCH=wasm $(GO) build -ldflags="-s -w" -trimpath -o $@ ./repjs

.PHONY: build/repjs.debug
## Build WebAssembly target, unstripped
build/repjs.debug:
	GOOS=js GOARCH=wasm $(GO) build -o $@ ./repjs

## Build Gzip-compressed WebAssembly target
build/repjs.gz: build/repjs
	gzip --best -f -k $<

## Build Brotli-compressed WebAssembly target
build/repjs.br: build/repjs
	brotli -f $<

.DEFAULT_GOAL := help
.PHONY: help
help:
	@awk '/^[a-zA-Z_\/\.0-9-]+:/ {        \
        nb = sub( /^## /, "", helpMsg );  \
        if (nb)                           \
            print  $$1 "\t" helpMsg;      \
    }                                     \
    { helpMsg = $$0 }' $(MAKEFILE_LIST) | \
	column -ts $$'\t' |                   \
	grep --color '^[^ ]*'

