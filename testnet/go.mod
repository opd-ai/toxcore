module github.com/opd-ai/toxcore/testnet

go 1.24.0

toolchain go1.24.12

require (
	github.com/opd-ai/toxcore v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.9.4
)

replace github.com/opd-ai/toxcore => ../

require (
	github.com/cretz/bine v0.2.0 // indirect
	github.com/flynn/noise v1.1.0 // indirect
	github.com/go-i2p/i2pkeys v0.33.92 // indirect
	github.com/go-i2p/onramp v0.33.92 // indirect
	github.com/go-i2p/sam3 v0.33.92 // indirect
	github.com/pion/opus v0.0.0-20250902022847-c2c56b95f05c // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtp v1.8.22 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
)
