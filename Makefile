SRC_DIR := src
DEST_DIR := dist
PRODUCT := foldergal

.PHONY: clean, run, build, build-all

build: $(DEST_DIR)/$(PRODUCT)

$(DEST_DIR)/$(PRODUCT): $(SRC_DIR)/*.go
	go build -v -o $@ $?

clean:
	rm -rf $(DEST_DIR)/*

run: build
	@. .env; ./$(DEST_DIR)/$(PRODUCT)

build-all: build
	GOOS=windows GOARCH=amd64 go build -v -o $(DEST_DIR)/$(PRODUCT).exe $(SRC_DIR)/*.go
	GOOS=linux GOARCH=amd64 go build -v -o $(DEST_DIR)/$(PRODUCT)-linux $(SRC_DIR)/*.go
	GOOS=freebsd GOARCH=amd64 go build -v -o $(DEST_DIR)/$(PRODUCT)-freebsd $(SRC_DIR)/*.go
	GOOS=linux GOARCH=arm GOARM=7 go build -v -o $(DEST_DIR)/$(PRODUCT)-pi $(SRC_DIR)/*.go

