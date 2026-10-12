#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo '[构建] 准备目标平台的 Pi 独立运行环境'
python3 scripts/prepare-pi.py
echo '[构建] 安装前端锁定依赖'
pnpm --dir frontend install --frozen-lockfile
echo '[构建] 构建前端'
pnpm --dir frontend build
echo '[构建] 编译 Go 程序'
if [[ $# -eq 0 ]]; then
  go build -o mywebscrcpy .
else
  go build "$@" .
fi
