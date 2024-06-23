CC = gcc
GZIP = gzip -9
BUILD_DIR = build
MAN_DIR = /usr/local/share/man
BIN_DIR = /usr/local/bin
SECTION = 1
SRC_FILES = $(wildcard src/*.c)
NROFF_FILES = $(wildcard doc/*.nroff)
MAN_FILES = $(NROFF_FILES:doc/%.nroff=$(BUILD_DIR)/%)
BIN_FILES = $(wildcard bin/*)
MAN_GZ_FILES = $(MAN_FILES:%=%.gz)
OBJ_FILES = $(SRC_FILES:src/%.c=$(BUILD_DIR)/%.o)


all: doc $(BUILD_DIR)/salsa-handler

doc: $(MAN_GZ_FILES)

$(BUILD_DIR)/salsa-handler: $(OBJ_FILES)
	$(CC) $(OBJ_FILES) -o bin/salsa-handler

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(BUILD_DIR)/%.gz: doc/%.nroff $(BUILD_DIR)
	$(GZIP) -c $< > $@

$(BUILD_DIR)/%.o: src/%.c $(BUILD_DIR)
	$(CC) -c $< -o $@

install: install-man install-bin

install-bin: all
	install -m 755 $(BIN_FILES) $(BIN_DIR)

install-man: $(MAN_GZ_FILES) all
	install -d $(MAN_DIR)/man$(SECTION)
	install -m 644 $(MAN_GZ_FILES) $(MAN_DIR)/man$(SECTION)

clean:
	rm -rf $(BUILD_DIR)
