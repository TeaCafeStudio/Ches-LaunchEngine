#!/usr/bin/env sh
set -eu

target="${1:-all}"
case "$target" in
  all|cle|cle-debug) ;;
  *)
    echo "usage: $0 [all|cle|cle-debug]" >&2
    exit 2
    ;;
esac

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$root"

build_program() {
  program="$1"
  command_path="./cmd/$program"
  for platform in windows-amd64 windows-arm64 linux-amd64 linux-arm64 darwin-amd64 darwin-arm64; do
    os="${platform%-*}"
    arch="${platform#*-}"
    extension=""
    if [ "$os" = windows ]; then extension=".exe"; fi
    output="dist/$platform/$program$extension"
    mkdir -p "dist/$platform"
    echo "Building $program for $os/$arch"
    GOOS="$os" GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -o "$output" "$command_path"
  done
}

if [ "$target" = all ] || [ "$target" = cle ]; then
  build_program cle
fi
if [ "$target" = all ] || [ "$target" = cle-debug ]; then
  build_program cle-debug
fi
