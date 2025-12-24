#!/bin/bash
# Bittap 延迟套利验证器 - GCP 部署脚本
# 使用方法: chmod +x install.sh && ./install.sh

set -e

APP_NAME="bittap-validator"
INSTALL_DIR="/opt/$APP_NAME"
USER="validator"

echo "=== Bittap Validator 部署脚本 ==="

# 创建用户（如不存在）
if ! id "$USER" &>/dev/null; then
    echo "[1/5] 创建用户 $USER..."
    sudo useradd -r -s /bin/false $USER
else
    echo "[1/5] 用户 $USER 已存在"
fi

# 创建安装目录
echo "[2/5] 创建安装目录..."
sudo mkdir -p $INSTALL_DIR/output
sudo cp validator $INSTALL_DIR/
sudo cp config.yaml $INSTALL_DIR/
sudo chown -R $USER:$USER $INSTALL_DIR
sudo chmod +x $INSTALL_DIR/validator

# 安装 systemd 服务
echo "[3/5] 安装 systemd 服务..."
sudo cp bittap-validator.service /etc/systemd/system/
sudo systemctl daemon-reload

# 启动服务
echo "[4/5] 启动服务..."
sudo systemctl enable $APP_NAME
sudo systemctl start $APP_NAME

# 检查状态
echo "[5/5] 检查服务状态..."
sleep 2
sudo systemctl status $APP_NAME --no-pager

echo ""
echo "=== 部署完成 ==="
echo "查看日志: sudo journalctl -u $APP_NAME -f"
echo "输出目录: $INSTALL_DIR/output/"
echo "停止服务: sudo systemctl stop $APP_NAME"
