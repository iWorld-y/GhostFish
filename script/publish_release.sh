#!/usr/bin/env bash
# 校验全部资产后创建草稿；确认 GitHub 收齐资产后才公开。
set -euo pipefail
assets_dir="$1"
version="$2"
tag="$3"
expected=(
  "GhostFish-${version}-arm64.dmg"
  "GhostFish-${version}-amd64.dmg"
  "ghostfish-linux-amd64"
  "ghostfish-windows-amd64.exe"
)

if [[ ! "$version" =~ ^[1-9][0-9]*$ || "$tag" != "v$version" ]]; then
  echo "无效版本或标签：$version / $tag" >&2
  exit 1
fi
actual="$(find "$assets_dir" -type f -print | sed 's#.*/##' | LC_ALL=C sort)"
expected_lines="$(printf '%s\n' "${expected[@]}" | LC_ALL=C sort)"
if [[ "$actual" != "$expected_lines" ]]; then
  echo "发布资产不符合预期：$actual" >&2
  exit 1
fi
files=()
for name in "${expected[@]}"; do
  file="$assets_dir/$name"
  test -s "$file"
  files+=("$file")
done

response="$(mktemp)"
trap 'rm -f "$response"' EXIT
# 列表接口在具备推送权限时包含草稿；任何查询错误都必须停止，不能当作不存在。
if ! gh api --paginate --slurp "repos/${GITHUB_REPOSITORY}/releases?per_page=100" > "$response"; then
  echo "无法读取 Release 列表，停止发布。" >&2
  exit 1
fi
if ! jq -e --arg tag "$tag" '[.[][] | select(.tag_name == $tag)] | length == 0' "$response" >/dev/null; then
  echo "Release $tag 已存在，或列表响应无效，拒绝覆盖。" >&2
  exit 1
fi

gh release create "$tag" "${files[@]}" --repo "$GITHUB_REPOSITORY" \
  --verify-tag --draft --title "GhostFish ${tag}" \
  --notes "macOS DMG 使用临时签名，尚未经过 Apple 公证。安装步骤请参阅 README。"
gh api --paginate --slurp "repos/${GITHUB_REPOSITORY}/releases?per_page=100" > "$response"
expected_json="$(printf '%s\n' "${expected[@]}" | jq -R . | jq -s 'sort')"
if ! jq -e --arg tag "$tag" --argjson expected "$expected_json" \
  '[.[][] | select(.tag_name == $tag)] | length == 1 and .[0].draft == true and (.[0].assets | length) == ($expected | length) and ([.[0].assets[] | select(.state == "uploaded") | .name] | sort) == $expected' \
  "$response" >/dev/null; then
  echo "草稿 Release 的资产不完整，保留草稿以便检查。" >&2
  exit 1
fi
gh release edit "$tag" --repo "$GITHUB_REPOSITORY" --draft=false
