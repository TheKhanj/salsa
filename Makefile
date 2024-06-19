GZIP = gzip -9
BUILD_DIR = build
MAN_DIR = /usr/local/share/man
BIN_DIR = /usr/local/bin
SECTION = 1
SRC_FILES = $(wildcard doc/*.nroff)
MAN_FILES = $(SRC_FILES:doc/%.nroff=$(BUILD_DIR)/%)
BIN_FILES = $(wildcard bin/*)
MAN_GZ_FILES = $(MAN_FILES:%=%.gz)

all: doc

doc: $(MAN_GZ_FILES)

$(BUILD_DIR)/%.gz: doc/%.nroff
	mkdir -p $(BUILD_DIR)
	$(GZIP) -c $< > $@

install: install-man install-bin

install-bin:
	install -m 755 $(BIN_FILES) $(BIN_DIR)

install-man: $(MAN_GZ_FILES)
	install -d $(MAN_DIR)/man$(SECTION)
	install -m 644 $(MAN_GZ_FILES) $(MAN_DIR)/man$(SECTION)


clean:
	rm -rf $(BUILD_DIR)
