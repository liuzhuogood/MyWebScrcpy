#!/usr/bin/env bash
#
# 生产环境部署脚本 — MyWebScrcpy
#
# 根据 docs/environments/prod.md 资产文档生成。
# 部署方式：替换二进制 + 重启 systemd 服务。
#
# 用法：
#   1. 先在本机构建：GOOS=linux GOARCH=amd64 go build -o mywebscrcpy-linux-amd64 .
#   2. 运行部署：
#      DEPLOY_HOST=10.0.0.6 DEPLOY_USER=liuzhuo \
#        bash scripts/deploy/deploy-prod.sh
#
# 必填环境变量：
#   DEPLOY_HOST   服务器 IP（默认 10.0.0.6）
#   DEPLOY_USER   SSH 登录用户（默认 liuzhuo）
#
# 可选环境变量：
#   SSH_PORT      SSH 端口（默认 22）
#   DEPLOY_PATH   服务器上的部署目录（默认 /opt/mywebscrcpy）
#   BINARY_NAME   二进制文件名（默认 mywebscrcpy-linux-amd64）
#   SERVICE_NAME  systemd 服务名（默认 mywebscrcpy.service）
#
set -euo pipefail

# ---- 颜色输出 ----
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()   { echo -e "${GREEN}[部署]${NC} $*"; }
warn()  { echo -e "${YELLOW}[警告]${NC} $*"; }
error() { echo -e "${RED}[错误]${NC} $*" >&2; }

# ---- 参数（默认值来自 prod.md）----
DEPLOY_HOST="${DEPLOY_HOST:-10.0.0.6}"
DEPLOY_USER="${DEPLOY_USER:-liuzhuo}"
SSH_PORT="${SSH_PORT:-22}"
DEPLOY_PATH="${DEPLOY_PATH:-/opt/mywebscrcpy}"
BINARY_NAME="${BINARY_NAME:-mywebscrcpy-linux-amd64}"
SERVICE_NAME="${SERVICE_NAME:-mywebscrcpy.service}"

# ---- 校验本地二进制 ----
LOCAL_BINARY="./${BINARY_NAME}"
if [[ ! -f "$LOCAL_BINARY" ]]; then
  error "本地二进制文件不存在：${LOCAL_BINARY}"
  error "请先构建：GOOS=linux GOARCH=amd64 go build -o ${BINARY_NAME} ."
  exit 1
fi

log "目标环境：生产环境（PROD）"
log "目标服务器：${DEPLOY_USER}@${DEPLOY_HOST}:${SSH_PORT}"
log "部署目录：${DEPLOY_PATH}"
log "服务名称：${SERVICE_NAME}"

# ---- 生产环境二次确认 ----
warn "⚠️ 即将在生产环境执行部署，操作会重启服务。"
read -r -p "确认继续？输入 yes 继续，其他取消: " confirm
[[ "$confirm" == "yes" ]] || { warn "已取消部署。"; exit 0; }

# ---- 第 1 步：上传二进制到用户目录（/opt/ 需要 root 权限）----
REMOTE_TMP="/home/${DEPLOY_USER}/${BINARY_NAME}.new"
log "上传二进制文件到服务器（${REMOTE_TMP}）..."
scp -P "$SSH_PORT" "$LOCAL_BINARY" "${DEPLOY_USER}@${DEPLOY_HOST}:${REMOTE_TMP}"

# ---- 第 2 步：远程替换并重启 ----
log "在服务器上替换二进制并重启服务..."
ssh -p "$SSH_PORT" "${DEPLOY_USER}@${DEPLOY_HOST}" bash -s <<REMOTE_EOF
  set -euo pipefail

  echo "[远程] 备份当前二进制..."
  if [[ -f "${DEPLOY_PATH}/${BINARY_NAME}" ]]; then
    sudo cp "${DEPLOY_PATH}/${BINARY_NAME}" "${DEPLOY_PATH}/${BINARY_NAME}.bak.\$(date +%Y%m%d%H%M%S)"
    echo "[远程] 已备份到 ${BINARY_NAME}.bak.\$(date +%Y%m%d%H%M%S)"
  fi

  echo "[远程] 替换二进制..."
  sudo chmod +x "${REMOTE_TMP}"
  sudo mv "${REMOTE_TMP}" "${DEPLOY_PATH}/${BINARY_NAME}"

  echo "[远程] 重启 systemd 服务..."
  sudo systemctl restart "${SERVICE_NAME}"

  echo "[远程] 等待服务启动（3 秒）..."
  sleep 3

  echo "[远程] 检查服务状态..."
  sudo systemctl status "${SERVICE_NAME}" --no-pager || true

  echo "[远程] 检查进程是否运行..."
  if pgrep -f "${BINARY_NAME}" >/dev/null 2>&1; then
    echo "[远程] ✅ 进程运行中"
  else
    echo "[远程] ❌ 进程未运行"
    exit 1
  fi
REMOTE_EOF

# ---- 第 3 步：健康检查 ----
HEALTH_URL="https://${DEPLOY_HOST}:8080"
log "健康检查：GET ${HEALTH_URL}"
sleep 2
if curl -sk --max-time 15 "${HEALTH_URL}" >/dev/null 2>&1; then
  log "✅ 健康检查通过，服务正常运行。"
else
  error "❌ 健康检查失败：${HEALTH_URL} 未在 15 秒内响应。"
  warn "请检查服务日志：ssh ${DEPLOY_USER}@${DEPLOY_HOST} journalctl -u ${SERVICE_NAME} -n 50"
  warn "如需回滚，可使用备份二进制文件手动恢复。"
  exit 1
fi

# ---- 完成 ----
log "✅ 部署完成：生产环境 @ ${DEPLOY_HOST}"
log "访问地址：https://${DEPLOY_HOST}:8080"
