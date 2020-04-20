SRC_DIR := src
DEST_DIR := dist
PRODUCT := foldergal

.PHONY: clean, run, build

build: $(DEST_DIR)/$(PRODUCT)

$(DEST_DIR)/$(PRODUCT): $(SRC_DIR)/main.go
	go build -o $@ $?

clean:
	rm -rf $(DEST_DIR)/*

run: build
	@./$(DEST_DIR)/$(PRODUCT)
