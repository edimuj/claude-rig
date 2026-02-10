BINARY := claude-rig
VERSION := $(shell grep 'const version' cmd/claude-rig/main.go | cut -d'"' -f2)

.PHONY: build install clean

build:
	go build -o $(BINARY) ./cmd/claude-rig

install: build
	mv $(BINARY) $(GOPATH)/bin/ 2>/dev/null || mv $(BINARY) ~/go/bin/

clean:
	rm -f $(BINARY)
