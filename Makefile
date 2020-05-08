SRC_DIR = .
SOURCES = $(shell find $(SRC_DIR) -name '*.go')
DEST_DIR = dist
RES_DIR = res
PRODUCT = foldergal
VERSION = "2.0.3"
PLATFORMS = -mac .exe -linux -freebsd -pi
PRODUCT_FILES := $(PLATFORMS:%=$(DEST_DIR)/$(PRODUCT)%)
TIME := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
FLAGS := -ldflags="-X 'main.BuildTimestamp=$(TIME)' -X 'main.BuildVersion=$(VERSION)'"

.PHONY: clean run build build-all compress-all rerun rebuild zip-all favicon

build: $(DEST_DIR) $(DEST_DIR)/$(PRODUCT)

$(DEST_DIR):
	test -d $@ || mkdir $@

$(DEST_DIR)/$(PRODUCT): $(SOURCES)
	go generate
	go build -v $(FLAGS) -o $@

clean: $(DEST_DIR)
	rm -rf $(DEST_DIR)/*

run: build
	./$(DEST_DIR)/$(PRODUCT) --config config.json

build-all: $(DEST_DIR) $(PRODUCT_FILES)

$(DEST_DIR)/$(PRODUCT)-mac: $(SOURCES)
	go generate
	GOOS=darwin GOARCH=amd64 go build -v $(FLAGS) -o $@
$(DEST_DIR)/$(PRODUCT).exe: $(SOURCES)
	go generate
	GOOS=windows GOARCH=amd64 go build -v $(FLAGS) -o $@
$(DEST_DIR)/$(PRODUCT)-linux: $(SOURCES)
	go generate
	GOOS=linux GOARCH=amd64 go build -v $(FLAGS) -o $@
$(DEST_DIR)/$(PRODUCT)-freebsd: $(SOURCES)
	go generate
	GOOS=freebsd GOARCH=amd64 go build -v $(FLAGS) -o $@
$(DEST_DIR)/$(PRODUCT)-pi: $(SOURCES)
	go generate
	GOOS=linux GOARCH=arm GOARM=7 go build -v $(FLAGS) -o $@

rerun: clean run

rebuild: clean build

compress-all: $(PRODUCT_FILES)
	upx --brute $?

zip-all: build-all
	cd $(DEST_DIR); find . -type f -not -name "*.zip" -and -not -name ".*" -exec zip "{}.zip" "{}" \;

$(RES_DIR)/favicon.png: $(RES_DIR)/favicon.svg
	cd $(RES_DIR); ffmpeg -hide_banner -loglevel quiet -y -i favicon.svg favicon.png

$(RES_DIR)/favicon.ico: $(RES_DIR)/favicon.png
	cd $(RES_DIR); convert favicon.png -define icon:auto-resize=64,48,32,16 favicon.ico

favicon: $(RES_DIR)/favicon.ico
	cd $(RES_DIR); go-bindata favicon.ico
