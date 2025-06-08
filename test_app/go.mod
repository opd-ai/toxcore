module test

go 1.24.1

replace github.com/opd-ai/toxcore => ../

require github.com/opd-ai/toxcore v0.0.0-20250608164101-653911b19e05

require (
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
)
