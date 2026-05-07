#!/bin/sh
set -eu

NFPM=${NFPM:-nfpm}
output=${1:-dist/hexone_windows_amd64.msix}

if ! command -v "$NFPM" >/dev/null 2>&1; then
  echo "nfpm is required to build the Windows MSIX package" >&2
  exit 1
fi

HEXONE_VERSION=${HEXONE_VERSION:-$(sh packaging/derive_version.sh display)}
HEXONE_TAG_VERSION=${HEXONE_TAG_VERSION:-$(sh packaging/derive_version.sh tag)}
HEXONE_SEMVER=${HEXONE_SEMVER:-${HEXONE_TAG_VERSION#v}}
HEXONE_MSIX_PUBLISHER=${HEXONE_MSIX_PUBLISHER:-CN=Ramunas Cvirka}
HEXONE_MSIX_PUBLISHER_DISPLAY_NAME=${HEXONE_MSIX_PUBLISHER_DISPLAY_NAME:-Ramunas Cvirka}

export HEXONE_VERSION
export HEXONE_TAG_VERSION
export HEXONE_SEMVER
export HEXONE_MSIX_PUBLISHER
export HEXONE_MSIX_PUBLISHER_DISPLAY_NAME

make build-windows-pdfium
go run ./tools/msixassets dist/msix-assets
mkdir -p "$(dirname "$output")"
"$NFPM" package -f packaging/windows/nfpm-msix.yaml -p msix -t "$output"
