#!/usr/bin/env bash
# 检查指定架构的应用包及只读挂载的 DMG 内容。
set -euo pipefail
version="$1"
arch="$2"
case "$arch" in
  arm64) macho_arch=arm64 ;;
  amd64) macho_arch=x86_64 ;;
  *) echo "不支持的 macOS 架构：$arch" >&2; exit 1 ;;
esac
app="dist/darwin-${arch}/GhostFish.app"
dmg="dist/darwin-${arch}/ghostfish-${version}-${arch}.dmg"
executable="$app/Contents/MacOS/GhostFish"

test -x "$executable"
test -s "$app/Contents/Resources/AppIcon.icns"
test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$app/Contents/Info.plist")" = "io.github.iworld-y.ghostfish"
test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$app/Contents/Info.plist")" = "$version"
test "$(/usr/libexec/PlistBuddy -c 'Print :LSMinimumSystemVersion' "$app/Contents/Info.plist")" = "12.0"
test "$(/usr/libexec/PlistBuddy -c 'Print :LSUIElement' "$app/Contents/Info.plist")" = "true"
test "$("$executable" -version)" = "ghostfish v${version}"
test "$(lipo -archs "$executable")" = "$macho_arch"
codesign --verify --deep --strict --verbose=2 "$app"
hdiutil verify "$dmg"

mount_point="$(mktemp -d)"
cleanup() {
  hdiutil detach "$mount_point" >/dev/null 2>&1 || true
  rmdir "$mount_point" 2>/dev/null || true
}
trap cleanup EXIT
hdiutil attach -nobrowse -readonly -mountpoint "$mount_point" "$dmg"
test -x "$mount_point/GhostFish.app/Contents/MacOS/GhostFish"
test -L "$mount_point/Applications"
test "$(readlink "$mount_point/Applications")" = "/Applications"
test -s "$mount_point/GhostFish.app/Contents/Resources/AppIcon.icns"
test "$("$mount_point/GhostFish.app/Contents/MacOS/GhostFish" -version)" = "ghostfish v${version}"
test "$(lipo -archs "$mount_point/GhostFish.app/Contents/MacOS/GhostFish")" = "$macho_arch"
codesign --verify --deep --strict --verbose=2 "$mount_point/GhostFish.app"
