
go install mvdan.cc/gofumpt@latest
find . -name '*.go' -exec gofumpt -w -s {} \;
