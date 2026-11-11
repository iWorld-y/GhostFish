#!/usr/bin/env bash
# 校验受版本管理的应用版本，以及发布事件的标签。
set -euo pipefail

version="$(jq -er '.version | strings' mygo.json)"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "应用版本格式错误：$version" >&2
  exit 1
fi

tag=""
is_release=false
case "${GITHUB_EVENT_NAME}" in
  push)
    if [[ ! "${GITHUB_REF}" =~ ^refs/tags/v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      echo "标签格式错误：${GITHUB_REF}" >&2
      exit 1
    fi
    tag="${GITHUB_REF#refs/tags/}"
    if [[ "$tag" != "v$version" ]]; then
      echo "标签与应用版本不一致：$tag / $version" >&2
      exit 1
    fi
    is_release=true
    ;;
  workflow_dispatch) ;;
  *) echo "不支持的触发事件：${GITHUB_EVENT_NAME}" >&2; exit 1 ;;
esac

{
  echo "version=$version"
  echo "tag=$tag"
  echo "is_release=$is_release"
} >> "${GITHUB_OUTPUT}"
