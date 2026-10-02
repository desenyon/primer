#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
version="v$(cat VERSION)"
case "$version" in *[!a-zA-Z0-9._+-]*) printf 'Invalid VERSION\n' >&2; exit 1 ;; esac
mkdir -p dist
work=$(mktemp -d "${TMPDIR:-/tmp}/primer-release.XXXXXX")
trap 'rm -rf "$work"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

for target in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
  platform=${target%/*}
  arch=${target#*/}
  printf 'Building %s %s\n' "$version" "$target"
  CGO_ENABLED=0 GOOS="$platform" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$work/primer" ./cmd/primer
  cp LICENSE "$work/LICENSE"
  COPYFILE_DISABLE=1 tar -czf "dist/primer_${version}_${platform}_${arch}.tar.gz" \
    -C "$work" primer LICENSE
done
cp install.sh dist/install.sh
(
  cd dist
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum primer_"$version"_*.tar.gz install.sh
  else
    shasum -a 256 primer_"$version"_*.tar.gz install.sh
  fi
) > dist/checksums.txt
printf 'Release assets: %s/dist\n' "$root"
