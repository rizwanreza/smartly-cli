#!/usr/bin/env bash
set -euo pipefail
binary="$1"
target_os="$2"
[[ "$target_os" == darwin ]] || exit 0
[[ -n "${MACOS_SIGN_P12:-}" ]] || exit 0
: "${MACOS_SIGN_KEYCHAIN:?Signing keychain is required}"
: "${MACOS_NOTARY_ISSUER_ID:?Notary issuer is required}"
: "${MACOS_NOTARY_KEY_ID:?Notary key ID is required}"
: "${MACOS_NOTARY_KEY:?Notary key is required}"
identity="$(security find-identity -v -p codesigning "$MACOS_SIGN_KEYCHAIN" | awk '/Developer ID Application/ {print $2; exit}')"
[[ -n "$identity" ]] || { echo 'Developer ID Application identity not found' >&2; exit 1; }
codesign --force --sign "$identity" --keychain "$MACOS_SIGN_KEYCHAIN" --options runtime --timestamp "$binary"
codesign --verify --strict --verbose=2 "$binary"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
printf '%s' "$MACOS_NOTARY_KEY" | base64 --decode > "$work/key.p8"
ditto -c -k "$binary" "$work/smartly.zip"
xcrun notarytool submit "$work/smartly.zip" --key "$work/key.p8" --key-id "$MACOS_NOTARY_KEY_ID" --issuer "$MACOS_NOTARY_ISSUER_ID" --wait
# codesign above validates both architectures. spctl app assessment is not
# appropriate for a standalone CLI (it rejects valid code that is not an app).
# Smoke-test the architecture this runner can execute before publishing.
case "$(uname -m)" in
  arm64) arch=arm64 ;;
  x86_64) arch=x86_64 ;;
esac
if lipo -archs "$binary" | tr ' ' '\n' | grep -qx "$arch"; then
  "$binary" --version
fi
