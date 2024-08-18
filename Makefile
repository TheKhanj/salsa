GZIP = gzip -9
BUILD_DIR = build
MAN_DIR = /usr/local/share/man
BIN_DIR = /usr/local/bin
SECTION = 1
SRC_FILES = $(wildcard *.go)
ROFF_FILES = $(wildcard doc/*.roff)
MAN_FILES = $(ROFF_FILES:doc/%.roff=$(BUILD_DIR)/%)
MAN_GZ_FILES = $(MAN_FILES:%=%.gz)
OBJ_FILES = $(SRC_FILES:src/%.c=$(BUILD_DIR)/%.o)


all: doc $(BUILD_DIR)/salsa

doc: $(MAN_GZ_FILES)

$(BUILD_DIR)/salsa: $(SRC_FILES)
	go build -o build/salsa

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(BUILD_DIR)/%.gz: doc/%.roff $(BUILD_DIR)
	$(GZIP) -c $< > $@

install: install-doc install-bin

install-bin: all
	install -m 755 build/salsa $(BIN_DIR)

install-doc: $(MAN_GZ_FILES) all
	install -d $(MAN_DIR)/man$(SECTION)
	install -m 644 $(MAN_GZ_FILES) $(MAN_DIR)/man$(SECTION)

clean:
	rm -rf $(BUILD_DIR)
