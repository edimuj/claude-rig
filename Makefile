BINARY := claude-rig
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build install clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/claude-rig

install: build
	mv $(BINARY) $(GOPATH)/bin/ 2>/dev/null || mv $(BINARY) ~/go/bin/

clean:
	rm -f $(BINARY)
