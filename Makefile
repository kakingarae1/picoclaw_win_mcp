VERSION    := 1.0.0
CONFIG_PKG := github.com/kakingarae1/picoclaw/pkg/config
LDFLAGS    := -ldflags "-X $(CONFIG_PKG).Version=$(VERSION) -s -w -H windowsgui"
GO         := CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go
OUT        := picoclaw.exe

.PHONY: all build deps clean

all: build

deps:
	go mod tidy && go mod download

build:
	$(GO) build $(LDFLAGS) -o $(OUT) ./cmd/picoclaw

build-dev:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
		-ldflags "-X $(CONFIG_PKG).Version=$(VERSION)-dev -s -w" \
		-o picoclaw-dev.exe ./cmd/picoclaw

clean:
	del /f $(OUT) picoclaw-dev.exe 2>nul || true
