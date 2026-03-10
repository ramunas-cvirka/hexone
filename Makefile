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
MACOS_DMG_STAGE := $(DIST_DIR)/$(APP)-macos-dmg-$(MACOS_ARCH)
MACOS_DMG := $(DIST_DIR)/$(APP)_macos_$(MACOS_ARCH).dmg
MACOS_CODESIGN_IDENTITY ?= -
MACOS_NOTARY_PROFILE ?=

ifeq ($(MACOS_CODESIGN_IDENTITY),-)
MACOS_APP_CODESIGN_FLAGS := --timestamp=none
MACOS_DMG_SIGN := false
else
MACOS_APP_CODESIGN_FLAGS := --options runtime
MACOS_DMG_SIGN := true
endif

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
		echo "build-macos requires a macOS host (CGO-enabled Gio build)."; \
		exit 1; \
	fi
	rm -rf "$(MACOS_STAGE)"
	mkdir -p "$(MACOS_CONTENTS)/MacOS" "$(MACOS_RESOURCES)"
	GOOS=darwin GOARCH=$(MACOS_ARCH) CGO_ENABLED=1 go build -o "$(MACOS_BIN)" $(CMD)
	cp "$(MACOS_PLIST)" "$(MACOS_CONTENTS)/Info.plist"
	cp protocols.yaml "$(MACOS_RESOURCES)/protocols.yaml"
	HEXONE_WRITE_DEFAULT_ICON_ICNS="$(MACOS_RESOURCES)/AppIcon.icns" "$(MACOS_BIN)"
	codesign --force --sign "$(MACOS_CODESIGN_IDENTITY)" $(MACOS_APP_CODESIGN_FLAGS) "$(MACOS_APP)"
	codesign -v --verbose=2 "$(MACOS_APP)"

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
	rm -rf "$(MACOS_DMG_STAGE)"
	mkdir -p "$(MACOS_DMG_STAGE)"
	ditto "$(MACOS_APP)" "$(MACOS_DMG_STAGE)/$(APP).app"
	ln -s /Applications "$(MACOS_DMG_STAGE)/Applications"
	hdiutil create -volname "$(APP)" -srcfolder "$(MACOS_DMG_STAGE)" -ov -format UDZO "$(MACOS_DMG)"
	@if [ "$(MACOS_DMG_SIGN)" = "true" ]; then \
		codesign --force --sign "$(MACOS_CODESIGN_IDENTITY)" "$(MACOS_DMG)"; \
	fi
	@if [ -n "$(strip $(MACOS_NOTARY_PROFILE))" ]; then \
		xcrun notarytool submit "$(MACOS_DMG)" --keychain-profile "$(MACOS_NOTARY_PROFILE)" --wait; \
		xcrun stapler staple "$(MACOS_DMG)"; \
	fi

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
