#!/bin/bash
# 部署脚本：将 latency-arbitrage-validator 部署到 GCP Tokyo VM
# 目标 IP: 35.243.114.46
# 使用前请确保：
#   1. 已配置 SSH 密钥或 gcloud CLI
#   2. 目标机器已安装 Go 1.22+

set -e

# ==== 配置区域 ====
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_HOST="35.243.114.46"
REMOTE_DIR="/opt/latency-validator"
SERVICE_NAME="latency-validator"
LOCAL_PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== 部署 latency-arbitrage-validator ==="
echo "目标: ${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR}"
echo "本地项目: ${LOCAL_PROJECT_DIR}"

# ==== Step 1: 在本地交叉编译 Linux amd64 二进制 ====
echo ""
echo "[1/4] 交叉编译 Linux amd64 二进制..."
cd "${LOCAL_PROJECT_DIR}"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ./deploy/validator ./cmd/validator

if [ ! -f "./deploy/validator" ]; then
    echo "编译失败！"
    exit 1
fi
echo "编译完成: ./deploy/validator ($(du -h ./deploy/validator | cut -f1))"

# ==== Step 2: 创建远程目录结构 ====
echo ""
echo "[2/4] 创建远程目录..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "mkdir -p ${REMOTE_DIR}/output && mkdir -p ${REMOTE_DIR}/logs"

# ==== Step 3: 上传文件 ====
echo ""
echo "[3/4] 上传文件..."
scp ./deploy/validator "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR}/validator"
scp ./config.yaml "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR}/config.yaml"
scp ./deploy/latency-validator.service "${REMOTE_USER}@${REMOTE_HOST}:/tmp/latency-validator.service"

# ==== Step 4: 配置 systemd 并启动服务 ====
echo ""
echo "[4/4] 配置 systemd 服务..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" << 'EOF'
sudo mv /tmp/latency-validator.service /etc/systemd/system/latency-validator.service
sudo chmod 644 /etc/systemd/system/latency-validator.service
sudo systemctl daemon-reload
sudo systemctl enable latency-validator
sudo systemctl restart latency-validator
sleep 2
sudo systemctl status latency-validator --no-pager || true
EOF

echo ""
echo "=== 部署完成 ==="
echo "查看日志: ssh ${REMOTE_USER}@${REMOTE_HOST} 'journalctl -u ${SERVICE_NAME} -f'"
echo "查看输出: ssh ${REMOTE_USER}@${REMOTE_HOST} 'ls -la ${REMOTE_DIR}/output/'"
