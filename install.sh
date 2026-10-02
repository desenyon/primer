#!/bin/sh
# Install a checksummed Primer release. No Go toolchain or elevated privileges.
set -eu

repo=desenyon/primer
version=${PRIMER_VERSION:-v0.1.0-alpha.2}
install_dir=${PRIMER_INSTALL_DIR:-"$HOME/.local/bin"}

fail() { printf 'Primer: %s\n' "$*" >&2; exit 1; }

case "$version" in
  v[0-9]*) ;;
  *) fail 'PRIMER_VERSION must be a version tag beginning with v.' ;;
esac
case "$version" in *[!a-zA-Z0-9._+-]*) fail 'Invalid version tag.' ;; esac
case "$(uname -s)" in
  Darwin) platform=darwin ;;
  Linux) platform=linux ;;
  *) fail 'Release installation supports macOS and Linux.' ;;
esac
case "$(uname -m)" in
  arm64|aarch64) arch=arm64 ;;
  x86_64|amd64) arch=amd64 ;;
  *) fail 'Release installation supports arm64 and amd64.' ;;
esac

for tool in curl tar mktemp chmod mv; do
  command -v "$tool" >/dev/null 2>&1 || fail "Required tool is unavailable: $tool"
done
if command -v sha256sum >/dev/null 2>&1; then
  checksum_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  checksum_tool=shasum
else
  fail 'SHA256 verification requires sha256sum or shasum.'
fi

asset="primer_${version}_${platform}_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/$version"
work=$(mktemp -d "${TMPDIR:-/tmp}/primer-install.XXXXXX")
staged=
cleanup() {
  if [ -n "$staged" ]; then rm -f "$staged"; fi
  rm -rf "$work"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

printf 'Downloading Primer %s for %s/%s\n' "$version" "$platform" "$arch"
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
  "$base/$asset" --output "$work/$asset"
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
  "$base/checksums.txt" --output "$work/checksums.txt"

expected=$(awk -v asset="$asset" '$2 == asset { print $1; count++ } END { if (count != 1) exit 1 }' "$work/checksums.txt") || fail 'Release checksum is missing or ambiguous.'
case "$expected" in *[!a-fA-F0-9]*|'') fail 'Invalid release checksum.' ;; esac
[ "${#expected}" -eq 64 ] || fail 'Invalid release checksum length.'
if [ "$checksum_tool" = sha256sum ]; then
  actual=$(sha256sum "$work/$asset" | awk '{ print $1 }')
else
  actual=$(shasum -a 256 "$work/$asset" | awk '{ print $1 }')
fi
[ "$actual" = "$expected" ] || fail 'Checksum mismatch. Nothing was installed.'

tar -xzf "$work/$asset" -C "$work" primer
[ -f "$work/primer" ] && [ ! -L "$work/primer" ] || fail 'Release does not contain a regular Primer binary.'
chmod 755 "$work/primer"
[ "$("$work/primer" --version)" = "primer $version" ] || fail 'Release binary version does not match its tag.'
[ ! -d "$install_dir/primer" ] || fail 'The installation target is a directory.'
mkdir -p "$install_dir"
staged=$(mktemp "$install_dir/.primer.XXXXXX")
cp "$work/primer" "$staged"
chmod 755 "$staged"
mv -f "$staged" "$install_dir/primer"
staged=

printf '\nInstalled %s\n' "$install_dir/primer"
case ":${PATH:-}:" in
  *":$install_dir:"*) printf 'Run primer inside your project repository.\n' ;;
  *) printf 'Add %s to PATH, then run primer inside your project repository.\n' "$install_dir" ;;
esac
