.PHONY: build run test clean build-linux build-macos build-windows build-all

APP := hexone
CMD := ./cmd/hexone
DIST_DIR := dist
LINUX_BIN := $(DIST_DIR)/$(APP)-linux-amd64
MACOS_BIN := $(DIST_DIR)/$(APP)-macos-amd64
WINDOWS_BIN := $(DIST_DIR)/$(APP)-windows-amd64.exe

ifeq ($(OS),Windows_NT)
BIN := $(APP).exe
BUILD_FLAGS := -ldflags="-H windowsgui"
else
BIN := $(APP)
BUILD_FLAGS :=
endif

build:
	go build $(BUILD_FLAGS) -o $(BIN) $(CMD)

build-linux: | $(DIST_DIR)
	@if [ "$$(go env GOHOSTOS)" != "linux" ]; then \
		echo "build-linux requires a Linux host (CGO-enabled Gio build)."; \
		exit 1; \
	fi
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o $(LINUX_BIN) $(CMD)

build-macos: | $(DIST_DIR)
	@if [ "$$(go env GOHOSTOS)" != "darwin" ]; then \
		echo "build-macos requires a macOS host (CGO-enabled Gio build)."; \
		exit 1; \
	fi
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -o $(MACOS_BIN) $(CMD)

build-windows: | $(DIST_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-H windowsgui" -o $(WINDOWS_BIN) $(CMD)

build-all: build-linux build-macos build-windows

$(DIST_DIR):
	mkdir -p $(DIST_DIR)

run:
	go run $(CMD)

test:
	go test ./...

clean:
	$(RM) $(BIN)
	$(RM) -r $(DIST_DIR)
