#!/bin/sh
set -eu

bin_path=$1
target=${2:-linux_amd64}
stage_dir=dist/goreleaser/linux-amd64
lib_dir=$stage_dir/lib
share_dir=$stage_dir/share

case "$target" in
  linux_amd64*) ;;
  *) exit 0 ;;
esac

if ! command -v patchelf >/dev/null 2>&1; then
  echo "patchelf is required for Linux GoReleaser builds" >&2
  exit 1
fi

mkdir -p "$lib_dir" "$share_dir/applications" "$share_dir/icons/hicolor/512x512/apps"

patchelf --force-rpath --set-rpath '$ORIGIN/lib' "$bin_path"

for lib in libxkbcommon-x11.so.0 libxcb-xkb.so.1; do
  path=$(ldconfig -p | awk -v lib="$lib" '$1 == lib { print $NF; exit }')
  if [ -z "$path" ]; then
    echo "missing required Linux runtime library: $lib" >&2
    exit 1
  fi

  cp -L "$path" "$lib_dir/$lib"
  patchelf --force-rpath --set-rpath '$ORIGIN' "$lib_dir/$lib"
done

sed \
  -e "s/@HEXONE_VERSION@/${HEXONE_VERSION:-${GORELEASER_CURRENT_TAG:-dev}}/g" \
  -e "s/@HEXONE_SEMVER@/${HEXONE_SEMVER:-${GORELEASER_CURRENT_TAG:-0.0.0}}/g" \
  packaging/linux/hexone.desktop > "$share_dir/applications/hexone.desktop"

HEXONE_WRITE_DESKTOP_ICON_PNG="$share_dir/icons/hicolor/512x512/apps/hexone.png" "$bin_path"
