BIN := rkmon-arm64
PREFIX ?= /usr/local
VERSION ?= $(shell git describe --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test fmt vet lint clean install

all: lint build

build:
	CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) .

install: build
	install -Dm 755 $(BIN) $(DESTDIR)$(PREFIX)/bin/rkmon

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

lint:
	test -z "$$(gofmt -l .)"
	go vet ./...
	golangci-lint run ./...

clean:
	rm -f $(BIN)
