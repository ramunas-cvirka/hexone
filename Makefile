.PHONY: build run test clean build-linux build-macos build-windows build-all package-linux package-macos package-windows package-all

APP := hexone
CMD := ./cmd/hexone
DIST_DIR := dist

LINUX_ARCH := amd64
LINUX_STAGE := $(DIST_DIR)/$(APP)-linux-$(LINUX_ARCH)
LINUX_BIN := $(LINUX_STAGE)/$(APP)
LINUX_LIB_DIR := $(LINUX_STAGE)/lib
LINUX_ZIP := $(DIST_DIR)/$(APP)_linux_$(LINUX_ARCH).zip

MACOS_ARCH := arm64
MACOS_STAGE := $(DIST_DIR)/$(APP)-macos-$(MACOS_ARCH)
MACOS_APP := $(MACOS_STAGE)/$(APP).app
MACOS_CONTENTS := $(MACOS_APP)/Contents
MACOS_BIN := $(MACOS_CONTENTS)/MacOS/$(APP)
MACOS_RESOURCES := $(MACOS_CONTENTS)/Resources
MACOS_PLIST := packaging/macos/Info.plist
MACOS_ICONSET := $(DIST_DIR)/AppIcon.iconset
MACOS_ICON_SOURCE := appicon/hexone_icon_art.png
MACOS_DMG := $(DIST_DIR)/$(APP)_macos_$(MACOS_ARCH).dmg

WINDOWS_ARCH := amd64
WINDOWS_STAGE := $(DIST_DIR)/$(APP)-windows-$(WINDOWS_ARCH)-portable
WINDOWS_BIN := $(WINDOWS_STAGE)/$(APP).exe
WINDOWS_ZIP := $(DIST_DIR)/$(APP)_windows_$(WINDOWS_ARCH)_portable.zip

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
	@if ! command -v patchelf >/dev/null 2>&1; then \
		echo "build-linux requires patchelf to set the Linux rpath."; \
		exit 1; \
	fi
	rm -rf "$(LINUX_STAGE)"
	mkdir -p "$(LINUX_STAGE)" "$(LINUX_LIB_DIR)" "$(LINUX_STAGE)/share/applications" "$(LINUX_STAGE)/share/icons/hicolor/512x512/apps"
	GOOS=linux GOARCH=$(LINUX_ARCH) CGO_ENABLED=1 go build -tags nowayland -o "$(LINUX_BIN)" $(CMD)
	patchelf --force-rpath --set-rpath '$$ORIGIN/lib' "$(LINUX_BIN)"
	@if [ "$$(patchelf --print-rpath "$(LINUX_BIN)")" != '$$ORIGIN/lib' ]; then \
		echo "failed to set Linux rpath on $(LINUX_BIN)"; \
		exit 1; \
	fi
	chmod +x "$(LINUX_BIN)"
	cp packaging/linux/hexone.desktop "$(LINUX_STAGE)/share/applications/hexone.desktop"
	cp appicon/hexone_icon_art.png "$(LINUX_STAGE)/share/icons/hicolor/512x512/apps/hexone.png"
	for lib in libxkbcommon-x11.so.0 libxcb-xkb.so.1; do \
		path=$$(ldconfig -p | awk -v lib="$$lib" '$$1 == lib { print $$NF; exit }'); \
		if [ -z "$$path" ]; then \
			echo "missing required Linux runtime library: $$lib"; \
			exit 1; \
		fi; \
		cp -L "$$path" "$(LINUX_LIB_DIR)/$$lib"; \
		patchelf --force-rpath --set-rpath '$$ORIGIN' "$(LINUX_LIB_DIR)/$$lib"; \
	done
	cp protocols.yaml "$(LINUX_STAGE)/protocols.yaml"

build-macos: | $(DIST_DIR)
	@if [ "$$(go env GOHOSTOS)" != "darwin" ]; then \
		echo "build-macos requires a macOS host with sips/iconutil (CGO-enabled Gio build)."; \
		exit 1; \
	fi
	rm -rf "$(MACOS_STAGE)" "$(MACOS_ICONSET)"
	mkdir -p "$(MACOS_CONTENTS)/MacOS" "$(MACOS_RESOURCES)"
	GOOS=darwin GOARCH=$(MACOS_ARCH) CGO_ENABLED=1 go build -o "$(MACOS_BIN)" $(CMD)
	cp "$(MACOS_PLIST)" "$(MACOS_CONTENTS)/Info.plist"
	cp protocols.yaml "$(MACOS_RESOURCES)/protocols.yaml"
	mkdir -p "$(MACOS_ICONSET)"
	sips -z 16 16 "$(MACOS_ICON_SOURCE)" --out "$(MACOS_ICONSET)/icon_16x16.png" >/dev/null
	sips -z 32 32 "$(MACOS_ICON_SOURCE)" --out "$(MACOS_ICONSET)/icon_16x16@2x.png" >/dev/null
	sips -z 32 32 "$(MACOS_ICON_SOURCE)" --out "$(MACOS_ICONSET)/icon_32x32.png" >/dev/null
	sips -z 64 64 "$(MACOS_ICON_SOURCE)" --out "$(MACOS_ICONSET)/icon_32x32@2x.png" >/dev/null
	sips -z 128 128 "$(MACOS_ICON_SOURCE)" --out "$(MACOS_ICONSET)/icon_128x128.png" >/dev/null
	sips -z 256 256 "$(MACOS_ICON_SOURCE)" --out "$(MACOS_ICONSET)/icon_128x128@2x.png" >/dev/null
	sips -z 256 256 "$(MACOS_ICON_SOURCE)" --out "$(MACOS_ICONSET)/icon_256x256.png" >/dev/null
	sips -z 512 512 "$(MACOS_ICON_SOURCE)" --out "$(MACOS_ICONSET)/icon_256x256@2x.png" >/dev/null
	sips -z 512 512 "$(MACOS_ICON_SOURCE)" --out "$(MACOS_ICONSET)/icon_512x512.png" >/dev/null
	sips -z 1024 1024 "$(MACOS_ICON_SOURCE)" --out "$(MACOS_ICONSET)/icon_512x512@2x.png" >/dev/null
	iconutil -c icns "$(MACOS_ICONSET)" -o "$(MACOS_RESOURCES)/AppIcon.icns"
	rm -rf "$(MACOS_ICONSET)"

build-windows: | $(DIST_DIR)
	rm -rf "$(WINDOWS_STAGE)"
	mkdir -p "$(WINDOWS_STAGE)"
	GOOS=windows GOARCH=$(WINDOWS_ARCH) CGO_ENABLED=0 go build -ldflags="-H windowsgui" -o "$(WINDOWS_BIN)" $(CMD)
	cp protocols.yaml "$(WINDOWS_STAGE)/protocols.yaml"

build-all: build-linux build-macos build-windows

package-linux: build-linux
	rm -f "$(LINUX_ZIP)"
	cd "$(LINUX_STAGE)" && zip -rq "../$(notdir $(LINUX_ZIP))" .

package-macos: build-macos
	rm -f "$(MACOS_DMG)"
	hdiutil create -volname "$(APP)" -srcfolder "$(MACOS_STAGE)" -ov -format UDZO "$(MACOS_DMG)"

package-windows: build-windows
	rm -f "$(WINDOWS_ZIP)"
	cd "$(WINDOWS_STAGE)" && zip -rq "../$(notdir $(WINDOWS_ZIP))" .

package-all: package-linux package-macos package-windows

$(DIST_DIR):
	mkdir -p $(DIST_DIR)

run:
	go run $(CMD)

test:
	go test ./...

clean:
	$(RM) $(BIN)
	$(RM) -r $(DIST_DIR)
