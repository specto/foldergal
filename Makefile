SRC_DIR := src
DEST_DIR := dist
RES_DIR := res
PRODUCT := foldergal
VERSION:="2.0.0"
TIME:=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
FLAGS:=-ldflags="-X 'main.BuildTimestamp=$(TIME)' -X 'main.BuildVersion=$(VERSION)'"

.PHONY: clean run build build-all compress-all rerun rebuild zip-all favicon

build: $(DEST_DIR)/$(PRODUCT)

$(DEST_DIR)/$(PRODUCT): $(SRC_DIR)/*.go
	go build -v $(FLAGS) -o $@ $?

clean:
	rm -rf $(DEST_DIR)/*

run: build
	./$(DEST_DIR)/$(PRODUCT) --config config.json

build-all:
	GOOS=darwin GOARCH=amd64 go build -v $(FLAGS) -o $(DEST_DIR)/$(PRODUCT)-mac $(SRC_DIR)/*.go
	GOOS=windows GOARCH=amd64 go build -v $(FLAGS) -o $(DEST_DIR)/$(PRODUCT).exe $(SRC_DIR)/*.go
	GOOS=linux GOARCH=amd64 go build -v $(FLAGS) -o $(DEST_DIR)/$(PRODUCT)-linux $(SRC_DIR)/*.go
	GOOS=freebsd GOARCH=amd64 go build -v $(FLAGS) -o $(DEST_DIR)/$(PRODUCT)-freebsd $(SRC_DIR)/*.go
	GOOS=linux GOARCH=arm GOARM=7 go build -v $(FLAGS) -o $(DEST_DIR)/$(PRODUCT)-pi $(SRC_DIR)/*.go

rerun: clean run

rebuild: clean build

compress-all: $(DEST_DIR)/$(PRODUCT)-mac $(DEST_DIR)/$(PRODUCT).exe $(DEST_DIR)/$(PRODUCT)-linux $(DEST_DIR)/$(PRODUCT)-pi $(DEST_DIR)/$(PRODUCT)-freebsd
	upx --brute $?

zip-all:
	cd ${DEST_DIR}; find . -type f -not -name "*.zip" -and -not -name ".*" -exec zip "{}.zip" "{}" \;

$(RES_DIR)/favicon.png: $(RES_DIR)/favicon.svg
	cd ${RES_DIR}; ffmpeg -hide_banner -loglevel quiet -y -i favicon.svg favicon.png

$(RES_DIR)/favicon.ico: $(RES_DIR)/favicon.png
	cd ${RES_DIR}; convert favicon.png -define icon:auto-resize=64,48,32,16 favicon.ico

favicon: $(RES_DIR)/favicon.ico
	cd ${RES_DIR}; go-bindata favicon.ico
