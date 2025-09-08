module github.com/opd-ai/toxcore/testnet

go 1.23.2

require github.com/opd-ai/toxcore v0.0.0-00010101000000-000000000000

replace github.com/opd-ai/toxcore => ../

require (
	github.com/flynn/noise v1.1.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
)
