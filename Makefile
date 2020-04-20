SRC_DIR := src
DEST_DIR := dist
TPLS := edit.html view.html
TPL_TARGS := $(patsubst %,$(DEST_DIR)/templates/%,$(TPLS))

.PHONY: clean, run, build

build: $(DEST_DIR)/templates $(TPL_TARGS) $(DEST_DIR)/main

$(DEST_DIR)/main: $(SRC_DIR)/main.go
	go build -o $@ $?

$(DEST_DIR)/templates:
	mkdir $(DEST_DIR)/templates

$(DEST_DIR)/templates/%.html: $(SRC_DIR)/templates/%.html
	cp $< $@

clean:
	rm -rf $(DEST_DIR)/*

run: build
	@./$(DEST_DIR)/main
