#!/bin/sh
# Assemble dist/wenda-tong from binaries and offline media that already exist.
# Missing chat-model weights are expected. Missing the lite binary, launcher,
# docreader image, or bge-m3 weights is an error.
set -eu
cd "$(dirname "$0")/.."

out="dist/wenda-tong"
rm -rf "$out"
mkdir -p "$out"

copy() {
  src="$1"
  name="$(basename "$src")"
  if [ ! -f "$src" ]; then
    echo "missing $src" >&2
    exit 1
  fi
  cp "$src" "$out/$name"
}

copy deploy/wenda-tong/config.yaml
copy deploy/wenda-tong/env
copy deploy/wenda-tong/PACKAGE.txt
copy deploy/wenda-tong/start.sh
copy deploy/wenda-tong/docreader.sh
copy deploy/wenda-tong/bootstrap.sh
copy deploy/wenda-tong/start.ps1

if [ -f deploy/wenda-tong/WeKnora-lite ]; then
  copy deploy/wenda-tong/WeKnora-lite
elif [ -f deploy/wenda-tong/WeKnora-lite.exe ]; then
  copy deploy/wenda-tong/WeKnora-lite.exe
else
  echo "missing deploy/wenda-tong/WeKnora-lite" >&2
  exit 1
fi

if [ -f deploy/wenda-tong/wenda-tong ]; then
  copy deploy/wenda-tong/wenda-tong
elif [ -f deploy/wenda-tong/wenda-tong.exe ]; then
  copy deploy/wenda-tong/wenda-tong.exe
else
  echo "missing deploy/wenda-tong/wenda-tong" >&2
  exit 1
fi

image="${DOCREADER_TAR:-deploy/wenda-tong/weknora-docreader-wenda-tong.tar}"
weights="${BGE_M3_ARCHIVE:-deploy/wenda-tong/bge-m3.tar}"
copy "$image"
copy "$weights"

{
  echo
  echo "SHA256"
  echo "------"
  if command -v shasum >/dev/null 2>&1; then
    (cd "$out" && shasum -a 256 -- *)
  else
    (cd "$out" && sha256sum -- *)
  fi
} >> "$out/PACKAGE.txt"

echo "wrote $out"
