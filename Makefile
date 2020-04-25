SRC_DIR := src
DEST_DIR := dist
PRODUCT := foldergal
VERSION:="2.0.0"
TIME:=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
FLAGS:=-ldflags="-X 'main.BuildTimestamp=$(TIME)' -X 'main.BuildVersion=$(VERSION)'"

.PHONY: clean run build build-all compress-all rerun rebuild zip-all

build: $(DEST_DIR)/$(PRODUCT)

$(DEST_DIR)/$(PRODUCT): $(SRC_DIR)/*.go
	go build -v $(FLAGS) -o $@ $?

clean:
	rm -rf $(DEST_DIR)/*

run: build
	@. .env; ./$(DEST_DIR)/$(PRODUCT)

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
