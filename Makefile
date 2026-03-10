.PHONY: build run test clean build-linux build-macos build-windows build-all package-linux package-macos package-windows package-all

APP := hexone
CMD := ./cmd/hexone
DIST_DIR := dist
VERSION_TOOL := ./packaging/derive_version.sh

APP_VERSION := $(shell sh $(VERSION_TOOL) display)
APP_SEMVER := $(shell sh $(VERSION_TOOL) semver)
APP_FILE_VERSION := $(shell sh $(VERSION_TOOL) file)
APP_FILE_VERSION_COMMAS := $(shell sh $(VERSION_TOOL) commas)
APP_COMMIT := $(shell sh $(VERSION_TOOL) commit)

GO_LDFLAGS_COMMON := -X hexone/buildinfo.Version=$(APP_VERSION) -X hexone/buildinfo.SemVersion=$(APP_SEMVER) -X hexone/buildinfo.Commit=$(APP_COMMIT)

LINUX_ARCH := amd64
LINUX_STAGE := $(DIST_DIR)/$(APP)-linux-$(LINUX_ARCH)
LINUX_BIN := $(LINUX_STAGE)/$(APP)
LINUX_LIB_DIR := $(LINUX_STAGE)/lib
LINUX_ZIP := $(DIST_DIR)/$(APP)_linux_$(LINUX_ARCH).zip
LINUX_DESKTOP_TEMPLATE := packaging/linux/hexone.desktop

MACOS_ARCH := arm64
MACOS_STAGE := $(DIST_DIR)/$(APP)-macos-$(MACOS_ARCH)
MACOS_APP := $(MACOS_STAGE)/$(APP).app
MACOS_CONTENTS := $(MACOS_APP)/Contents
MACOS_BIN := $(MACOS_CONTENTS)/MacOS/$(APP)
MACOS_RESOURCES := $(MACOS_CONTENTS)/Resources
MACOS_PLIST_TEMPLATE := packaging/macos/Info.plist
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
WINDOWS_RC_TEMPLATE := cmd/hexone/app_icon_windows.rc
WINDOWS_RC_RENDERED := $(DIST_DIR)/hexone_windows.generated.rc
WINDOWS_SYSO_RENDERED := $(DIST_DIR)/hexone_windows.generated.syso
WINDOWS_SYSO := cmd/hexone/hexone_windows.syso

ifeq ($(OS),Windows_NT)
BIN := $(APP).exe
GO_LDFLAGS_HOST := $(GO_LDFLAGS_COMMON) -H windowsgui
else
BIN := $(APP)
GO_LDFLAGS_HOST := $(GO_LDFLAGS_COMMON)
endif

GO_LDFLAGS_WINDOWS := $(GO_LDFLAGS_COMMON) -H windowsgui

build:
	go build -ldflags="$(GO_LDFLAGS_HOST)" -o $(BIN) $(CMD)

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
	GOOS=linux GOARCH=$(LINUX_ARCH) CGO_ENABLED=1 go build -tags nowayland -ldflags="$(GO_LDFLAGS_COMMON)" -o "$(LINUX_BIN)" $(CMD)
	patchelf --force-rpath --set-rpath '$$ORIGIN/lib' "$(LINUX_BIN)"
	@if [ "$$(patchelf --print-rpath "$(LINUX_BIN)")" != '$$ORIGIN/lib' ]; then \
		echo "failed to set Linux rpath on $(LINUX_BIN)"; \
		exit 1; \
	fi
	chmod +x "$(LINUX_BIN)"
	sed -e 's/@HEXONE_VERSION@/$(APP_VERSION)/g' -e 's/@HEXONE_SEMVER@/$(APP_SEMVER)/g' "$(LINUX_DESKTOP_TEMPLATE)" > "$(LINUX_STAGE)/share/applications/hexone.desktop"
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
	GOOS=darwin GOARCH=$(MACOS_ARCH) CGO_ENABLED=1 go build -ldflags="$(GO_LDFLAGS_COMMON)" -o "$(MACOS_BIN)" $(CMD)
	sed -e 's/@HEXONE_VERSION@/$(APP_VERSION)/g' -e 's/@HEXONE_SEMVER@/$(APP_SEMVER)/g' -e 's/@HEXONE_FILE_VERSION@/$(APP_FILE_VERSION)/g' "$(MACOS_PLIST_TEMPLATE)" > "$(MACOS_CONTENTS)/Info.plist"
	cp protocols.yaml "$(MACOS_RESOURCES)/protocols.yaml"
	HEXONE_WRITE_DEFAULT_ICON_ICNS="$(MACOS_RESOURCES)/AppIcon.icns" "$(MACOS_BIN)"
	codesign --force --sign "$(MACOS_CODESIGN_IDENTITY)" $(MACOS_APP_CODESIGN_FLAGS) "$(MACOS_APP)"
	codesign -v --verbose=2 "$(MACOS_APP)"

build-windows: | $(DIST_DIR)
	rm -rf "$(WINDOWS_STAGE)"
	mkdir -p "$(WINDOWS_STAGE)"
	@set -e; \
	rc_compiler=""; \
	for candidate in x86_64-w64-mingw32-windres windres; do \
		if command -v "$$candidate" >/dev/null 2>&1; then \
			rc_compiler="$$candidate"; \
			break; \
		fi; \
	done; \
	backup=""; \
	if [ -f "$(WINDOWS_SYSO)" ]; then \
		backup="$(DIST_DIR)/hexone_windows.syso.bak"; \
		cp "$(WINDOWS_SYSO)" "$$backup"; \
	fi; \
	cleanup() { \
		rm -f "$(WINDOWS_RC_RENDERED)" "$(WINDOWS_SYSO_RENDERED)"; \
		if [ -n "$$backup" ] && [ -f "$$backup" ]; then \
			mv "$$backup" "$(WINDOWS_SYSO)"; \
		fi; \
	}; \
	trap cleanup EXIT INT TERM; \
	if [ -n "$$rc_compiler" ]; then \
		sed -e 's/@HEXONE_VERSION@/$(APP_VERSION)/g' \
			-e 's/@HEXONE_FILE_VERSION@/$(APP_FILE_VERSION)/g' \
			-e 's/@HEXONE_FILE_VERSION_COMMAS@/$(APP_FILE_VERSION_COMMAS)/g' \
			"$(WINDOWS_RC_TEMPLATE)" > "$(WINDOWS_RC_RENDERED)"; \
		"$$rc_compiler" -i "$(WINDOWS_RC_RENDERED)" -o "$(WINDOWS_SYSO_RENDERED)" -O coff; \
		cp "$(WINDOWS_SYSO_RENDERED)" "$(WINDOWS_SYSO)"; \
	else \
		echo "warning: no windres found; using existing Windows resource without refreshed version metadata."; \
	fi; \
	GOOS=windows GOARCH=$(WINDOWS_ARCH) CGO_ENABLED=0 go build -ldflags="$(GO_LDFLAGS_WINDOWS)" -o "$(WINDOWS_BIN)" $(CMD); \
	cp protocols.yaml "$(WINDOWS_STAGE)/protocols.yaml"; \
	cleanup; \
	trap - EXIT INT TERM

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
	go run -ldflags="$(GO_LDFLAGS_HOST)" $(CMD)

test:
	go test ./...

clean:
	$(RM) $(BIN)
	$(RM) -r $(DIST_DIR)
