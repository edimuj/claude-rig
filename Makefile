BINARY := claude-rig
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build build-windows install clean run

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/claude-rig

install: build
	mv $(BINARY) $(GOPATH)/bin/ 2>/dev/null || mv $(BINARY) ~/go/bin/

run:
	@go run ./cmd/claude-rig $(ARGS)

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY).exe ./cmd/claude-rig

clean:
	rm -f $(BINARY) $(BINARY).exe
