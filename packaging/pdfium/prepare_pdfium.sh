#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
	echo "usage: prepare_pdfium.sh <linux-amd64|macos-arm64|windows-amd64> <destination>" >&2
	exit 1
fi

target="$1"
dest="$2"

case "$target" in
	linux-amd64)
		asset="pdfium-linux-x64.tgz"
		import_name="libpdfium.so"
		runtime_name="libpdfium.so"
		libs="-L\${libdir} -lpdfium"
		;;
	macos-arm64)
		asset="pdfium-mac-arm64.tgz"
		import_name="libpdfium.dylib"
		runtime_name="libpdfium.dylib"
		libs="-L\${libdir} -lpdfium"
		;;
	windows-amd64)
		asset="pdfium-win-x64.tgz"
		import_name="pdfium.dll.lib"
		runtime_name="pdfium.dll"
		libs="-L\${libdir} -l:pdfium.dll.lib"
		;;
	*)
		echo "unsupported pdfium target: $target" >&2
		exit 1
		;;
esac

tmp="$dest.tmp"
archive="$tmp/$asset"
extract="$tmp/extract"

rm -rf "$tmp" "$dest"
mkdir -p "$extract" "$dest/include" "$dest/lib" "$dest/bin" "$dest/lib/pkgconfig"

curl -fsSL "https://github.com/bblanchon/pdfium-binaries/releases/latest/download/$asset" -o "$archive"
tar -xzf "$archive" -C "$extract"

find_one() {
	find "$extract" -type f -name "$1" | head -n 1
}

header_path="$(find_one fpdfview.h)"
if [ -z "$header_path" ]; then
	echo "failed to locate fpdfview.h in $asset" >&2
	exit 1
fi

import_path="$(find_one "$import_name")"
if [ -z "$import_path" ]; then
	echo "failed to locate $import_name in $asset" >&2
	exit 1
fi

runtime_path="$(find_one "$runtime_name")"
if [ -z "$runtime_path" ]; then
	echo "failed to locate $runtime_name in $asset" >&2
	exit 1
fi

include_dir="$(dirname "$header_path")"
cp -R "$include_dir"/. "$dest/include/"
cp "$import_path" "$dest/lib/"

if [ "$runtime_name" != "$import_name" ]; then
	cp "$runtime_path" "$dest/bin/"
fi

prefix="$(cd "$dest" && pwd | sed 's#\\#/#g')"
cat > "$dest/lib/pkgconfig/pdfium.pc" <<EOF
prefix=$prefix
libdir=\${prefix}/lib
includedir=\${prefix}/include

Name: PDFium
Description: PDFium
Version: latest
Libs: $libs
Cflags: -I\${includedir}
EOF

rm -rf "$tmp"
