#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
BUILD_DIR=${BUILD_DIR:-"$ROOT_DIR/build"}
VERSION=${VERSION:-0.1.0}
FEED_DIR="$BUILD_DIR/feed/keenetic"

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
    return
  fi

  shasum -a 256 "$1" | awk '{print $1}'
}

package_size() {
  wc -c < "$1" | tr -d ' '
}

write_index() {
  package=$1
  destination=$2
  filename=$(basename "$package")

  mkdir -p "$destination"
  cp "$package" "$destination/$filename"

  {
    ar p "$package" control.tar.gz | tar -xzO ./control
    printf 'Filename: %s\n' "$filename"
    printf 'Size: %s\n' "$(package_size "$package")"
    printf 'SHA256sum: %s\n' "$(sha256_file "$package")"
    printf '\n'
  } > "$destination/Packages"

  gzip -c "$destination/Packages" > "$destination/Packages.gz"
}

rm -rf "$FEED_DIR"
mkdir -p "$FEED_DIR"
cp "$ROOT_DIR/scripts/install.sh" "$FEED_DIR/install.sh"

write_index \
  "$BUILD_DIR/hrbridge_${VERSION}_aarch64-3.10.ipk" \
  "$FEED_DIR/aarch64-k3.10"
write_index \
  "$BUILD_DIR/hrbridge_${VERSION}_mipsel-3.4.ipk" \
  "$FEED_DIR/mipselsf-k3.4"
write_index \
  "$BUILD_DIR/hrbridge_${VERSION}_mips-3.4.ipk" \
  "$FEED_DIR/mipssf-k3.4"

printf 'HydraBridge opkg feed: %s\n' "$FEED_DIR"
