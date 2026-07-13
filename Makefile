GZIP = gzip -9
BUILD_DIR = build
MAN_DIR = /usr/local/share/man
BIN_DIR = /usr/local/bin
SECTION = 1
SRC_FILES = $(wildcard *.go)
ROFF_FILES = $(wildcard doc/*.roff)
MAN_GZ_FILES = $(ROFF_FILES:doc/%.roff=doc/%.gz)

VERSION = $(shell git describe --tags --exact-match 2>/dev/null || echo -n dev)
LD_FLAGS = -X 'main.VERSION=$(VERSION)'
ifneq ($(VERSION),dev)
LD_FLAGS += -s -w
endif

all: doc $(BUILD_DIR)/salsa

doc: $(MAN_GZ_FILES)

$(BUILD_DIR)/salsa: $(SRC_FILES)
	go build \
		-trimpath \
		-ldflags "$(LD_FLAGS)" \
		-o $@

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

doc/%.gz: doc/%.roff
	$(GZIP) -c $< > $@

install: install-doc install-bin

install-bin: all
	install -m 755 $(BUILD_DIR)/salsa $(BIN_DIR)

install-doc: $(MAN_GZ_FILES) all
	install -d $(MAN_DIR)/man$(SECTION)
	install -m 644 $(MAN_GZ_FILES) $(MAN_DIR)/man$(SECTION)

clean:
	rm -rf $(BUILD_DIR) doc/*.gz
