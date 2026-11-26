#!/usr/bin/env bash
# 自动发布下一个 tag：
#   1. 通过 GitHub 代理前缀拉取仓库最新 release tag（失败则回退本地 git tag）
#   2. 默认 patch +1（v1.0.9 -> v1.0.10），--minor/--major 可调级
#   3. 在当前 HEAD 打注释 tag 并 push 到 origin
#
# 用法:
#   scripts/tag.sh            # patch +1
#   scripts/tag.sh --minor    # minor +1 (v1.1.0)
#   scripts/tag.sh --major    # major +1 (v2.0.0)
#   scripts/tag.sh --dry-run  # 只打印将要执行的动作，不 tag 不 push
set -euo pipefail

# GitHub API 代理前缀（与 internal/update 的 ProxyPrefix 保持一致）
PROXY="${GH_PROXY_PREFIX:-https://v4.gh-proxy.org/}"
REPO="wangjin/android-file-sync"

level="patch"
dry_run=0
for arg in "$@"; do
  case "$arg" in
    --major) level="major" ;;
    --minor) level="minor" ;;
    --patch) level="patch" ;;
    --dry-run) dry_run=1 ;;
    -h|--help) sed -n '2,12p' "$0"; exit 0 ;;
    *) echo "未知参数: $arg (支持 --major/--minor/--patch/--dry-run)" >&2; exit 1 ;;
  esac
done

# --- 1. 最新 tag：优先走代理查 GitHub Releases API，失败回退本地 ---
latest=""
api_url="${PROXY}https://api.github.com/repos/${REPO}/releases/latest"
if json=$(curl -fsSL --max-time 20 "$api_url" 2>/dev/null); then
  latest=$(printf '%s' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
fi
if [ -z "$latest" ]; then
  echo "代理获取 release 失败(${api_url})，回退本地 git tag" >&2
  git fetch --tags --quiet 2>/dev/null || true
  latest=$(git tag -l 'v[0-9]*' | sort -V | tail -1)
fi
[ -n "$latest" ] || { echo "找不到任何 v* tag，无法推导下一版" >&2; exit 1; }

# --- 2. 版本号 +1 ---
base=${latest#v}
major=${base%%.*}
rest=${base#*.}
minor=${rest%%.*}
patch=${rest#*.}
case "$level" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
esac
new="v${major}.${minor}.${patch}"

# 新 tag 若已存在（本地或远端竞态），递增到不冲突为止
while git rev-parse -q --verify "refs/tags/$new" >/dev/null; do
  echo "$new 已存在，自动 +1" >&2
  case "$level" in
    major) major=$((major + 1)); minor=0; patch=0 ;;
    minor) minor=$((minor + 1)); patch=0 ;;
    patch) patch=$((patch + 1)) ;;
  esac
  new="v${major}.${minor}.${patch}"
done

# --- 3. 打 tag 并 push ---
echo "最新: $latest  ->  新 tag: $new (level=$level)"
if [ "$dry_run" = 1 ]; then
  echo "[dry-run] git tag -a $new -m $new && git push origin $new"
  exit 0
fi

git tag -a "$new" -m "$new"
# push 直连 origin（gh-proxy 仅代理 API/下载，不支持 git push 鉴权）
git push origin "$new"
echo "✓ 已推送 $new"
