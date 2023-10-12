SRC_DIR = "."
SOURCES := $(shell find $(SRC_DIR) -type f -name 'main.go')
DEST_DIR = dist
RES_DIR = res
PRODUCT = foldergal
# Get version from last tag
VERSION := $(shell git describe --always --tags --dirty=-dev | sed 's/release\///')
PLATFORMS = -mac-intel -mac-arm .exe -linux -freebsd -pi -openwrt
PRODUCT_FILES := $(PLATFORMS:%=$(DEST_DIR)/$(PRODUCT)-$(VERSION)%)
TIME := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
FLAGS := -ldflags="-s -w -X 'main.BuildTimestamp=$(TIME)' -X 'main.BuildVersion=$(VERSION)'"


.PHONY: help
help: ## Show this help
	@egrep -h '\s##\s' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: $(DEST_DIR) $(DEST_DIR)/$(PRODUCT) ## Build for the current machine

$(DEST_DIR):
	test -d $@ || mkdir $@

$(DEST_DIR)/$(PRODUCT): $(SOURCES)
	go generate
	go build -v $(FLAGS) -o $@ $^

.PHONY: clean
clean: $(DEST_DIR) ## Clean all build artifacts
	go clean
	rm -rf $(DEST_DIR)/*

.PHONY: run
run: build ## Run with current config
	$(DEST_DIR)/$(PRODUCT) --config config.json

.PHONY: release
release: $(DEST_DIR) lint $(PRODUCT_FILES) ## Build all release binaries

$(DEST_DIR)/$(PRODUCT)-$(VERSION)-mac-intel: $(SOURCES)
	go generate
	GOOS=darwin GOARCH=amd64 go build -v $(FLAGS) -o $@ $^
$(DEST_DIR)/$(PRODUCT)-$(VERSION)-mac-arm: $(SOURCES)
	go generate
	GOOS=darwin GOARCH=arm64 go build -v $(FLAGS) -o $@ $^
$(DEST_DIR)/$(PRODUCT)-$(VERSION).exe: $(SOURCES)
	go generate
	GOOS=windows GOARCH=amd64 go build -v $(FLAGS) -o $@ $^
$(DEST_DIR)/$(PRODUCT)-$(VERSION)-linux: $(SOURCES)
	go generate
	GOOS=linux GOARCH=amd64 go build -v $(FLAGS) -o $@ $^
$(DEST_DIR)/$(PRODUCT)-$(VERSION)-freebsd: $(SOURCES)
	go generate
	GOOS=freebsd GOARCH=amd64 go build -v $(FLAGS) -o $@ $^
$(DEST_DIR)/$(PRODUCT)-$(VERSION)-pi: $(SOURCES)
	go generate
	GOOS=linux GOARCH=arm GOARM=7 go build -v $(FLAGS) -o $@ $^
$(DEST_DIR)/$(PRODUCT)-$(VERSION)-openwrt: $(SOURCES)
	go generate
	GOOS=linux GOARCH=mips GOMIPS=softfloat go build -v $(FLAGS) -o $@ $^

.PHONY: rerun
rerun: clean run ## Clean, build, run

.PHONY: rebuild
rebuild: clean build ## Clean and build

.PHONY: upx
upx: ## Compress all built binaries with upx
	cd $(DEST_DIR); upx *; true

.PHONY: zip
zip: ## Archive all binaries to zip files
	cd $(DEST_DIR); find . -type f -not -name "*.zip" -and -not -name ".*" -exec zip "{}.zip" "{}" \;

$(RES_DIR)/favicon.png: $(RES_DIR)/favicon.svg
	cd $(RES_DIR); ffmpeg -hide_banner -loglevel quiet -y -i favicon.svg favicon.png

$(RES_DIR)/favicon.ico: $(RES_DIR)/favicon.png
	cd $(RES_DIR); convert favicon.png -define icon:auto-resize=64,48,32,16 favicon.ico

.PHONY: test
test: lint ## Run go tests
	go test -coverprofile cover.out ./...
	go tool cover -func=cover.out
	# go tool cover -html=cover.out -o cover.html
	rm cover.out
	go vet -composites=false ./...
	govulncheck ./...
	gosec ./...

.PHONY: lint
lint: ## Run go-critic lint
	gocritic check -enableAll ./...
	staticcheck ./...

.PHONY: benchmark
benchmark: ## Run go benchmarks
	go test -bench=. ./...

.PHONY: all
all: test release upx zip ## Test, build and compress all release binaries

.PHONY: format
format: ## Format all go code
	go fmt ./...

.PHONY: update
update: ## Update go modules
	go get -u ./...

.PHONY: tags
tags: ## Create tags for all go code
	uctags **/*.go

.PHONY: version
version: ## Show current version
	@echo $(VERSION)

.PHONY: echo
echo:
	@echo VERSION: $(VERSION)
	@echo PLATFORMS: $(PLATFORMS)
	@echo SRC_DIR: $(SRC_DIR)
	@echo DEST_DIR: $(DEST_DIR)
	@echo FLAGS: $(FLAGS)
	@echo SOURCES: $(SOURCES)