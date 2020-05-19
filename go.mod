module roci.dev/replicache-client

go 1.12

require (
	github.com/attic-labs/noms v0.0.0
	github.com/google/uuid v1.1.1 // indirect
	github.com/lithammer/shortuuid v3.0.0+incompatible
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.18.0
	github.com/stretchr/testify v1.5.1
	golang.org/x/sys v0.0.0-20190712062909-fae7ac547cb7 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	roci.dev/diff-server v0.0.0
)

replace (
	github.com/attic-labs/noms v0.0.0 => github.com/whiten/noms v0.0.0-20200518183434-a7407d2d80d5
	roci.dev/diff-server v0.0.0 => github.com/whiten/diff-server v0.0.0-20200519002532-84dc15df357b
)
