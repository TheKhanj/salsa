GZIP = gzip -9
BUILD_DIR = build
MAN_DIR = /usr/local/share/man
BIN_DIR = /usr/local/bin
LIB_DIR = /usr/local/lib/salsa
SECTION = 1
SRC_FILES = $(wildcard src/*.c)
NROFF_FILES = $(wildcard doc/*.nroff)
MAN_FILES = $(NROFF_FILES:doc/%.nroff=$(BUILD_DIR)/%)
MAN_GZ_FILES = $(MAN_FILES:%=%.gz)
OBJ_FILES = $(SRC_FILES:src/%.c=$(BUILD_DIR)/%.o)
CC = gcc
CFLAGS = -DLIB_DIR=\"$(LIB_DIR)\" -DMAX_BACKENDS=256 -DMAX_STR_LEN=1024
# TODO: improve this
BINS = salsa scoreboard_handler round_robin_handler

all: doc $(BUILD_DIR)/salsa $(BUILD_DIR)/scoreboard-handler $(BUILD_DIR)/round-robin-handler

doc: $(MAN_GZ_FILES)

$(BUILD_DIR)/salsa: $(OBJ_FILES)
	$(CC) $(CFLAGS) build/salsa.o -o $@

$(BUILD_DIR)/scoreboard_handler: $(OBJ_FILES)
	$(CC) $(CFLAGS) build/scoreboard_handler.o -o $@

$(BUILD_DIR)/round_robin_handler: $(OBJ_FILES)
	$(CC) $(CFLAGS) build/round_robin_handler.o -o $@

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(BUILD_DIR)/%.gz: doc/%.nroff $(BUILD_DIR)
	$(GZIP) -c $< > $@

$(BUILD_DIR)/%.o: src/%.c $(BUILD_DIR)
	$(CC) $(CFLAGS) -c $< -o $@

install: install-man install-bin

install-bin: all
	install -m 755 bin/salsa $(BIN_DIR)
	install -m 755 bin/scoreboard-handler $(LIB_DIR)
	install -m 755 bin/round-robin-handler $(LIB_DIR)

install-man: $(MAN_GZ_FILES) all
	install -d $(MAN_DIR)/man$(SECTION)
	install -m 644 $(MAN_GZ_FILES) $(MAN_DIR)/man$(SECTION)

clean:
	rm -rf $(BUILD_DIR)
