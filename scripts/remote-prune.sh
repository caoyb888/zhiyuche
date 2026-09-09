#!/usr/bin/env bash
# 由 sync-remote.ps1 在远程执行：删除远程仍被 git 跟踪、但本地工作区已不存在的文件。
# 用法：bash remote-prune.sh <项目目录> <本地文件清单>
set -uo pipefail
dir=$1
list=$2
cd "$dir" || exit 0
export LC_ALL=C
git ls-files 2>/dev/null | sort > /tmp/zuche-remote-tracked.txt
tr -d '\r' < "$list" | sort > /tmp/zuche-sync-sorted.txt
stale=$(comm -23 /tmp/zuche-remote-tracked.txt /tmp/zuche-sync-sorted.txt)
# 安全阀：清单为空或差异过大（>200 或 >30%）时拒绝删除，避免清单出错时清空工作区
total=$(wc -l < /tmp/zuche-remote-tracked.txt)
n=$(printf '%s' "$stale" | grep -c . || true)
if [ "$n" -gt 200 ] || { [ "$total" -gt 0 ] && [ $((n * 100 / total)) -gt 30 ]; }; then
  echo "prune skipped: $n of $total tracked files would be removed (suspicious list)"; exit 0
fi
if [ -n "$stale" ]; then
  printf '%s\n' "$stale" | tr '\n' '\0' | xargs -0 -r rm -f
  echo "removed $(printf '%s\n' "$stale" | wc -l) stale file(s)"
fi
