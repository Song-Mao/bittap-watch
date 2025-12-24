# 部署指南 - GCP Tokyo VM

## 目标环境
- **IP**: 35.243.114.46
- **区域**: GCP Tokyo (asia-northeast1)
- **规格**: 4 核 CPU

## 前置条件

### 1. 本地环境
- Go 1.22+ 已安装
- SSH 密钥已配置（或使用 gcloud CLI）

### 2. 远程 VM 准备
```bash
# SSH 到 VM
gcloud compute ssh <instance-name> --zone=asia-northeast1-a
# 或直接 SSH
ssh root@35.243.114.46

# 确保系统时间同步（对延迟测量至关重要）
sudo timedatectl set-ntp true
timedatectl status

# 安装必要工具（如果需要）
sudo apt update && sudo apt install -y htop iotop
```

## 部署方式

### 方式一：使用部署脚本（推荐）
```powershell
# Windows PowerShell
cd F:\goproject\src\Bittap-watch

# 设置远程用户（默认 root）
$env:REMOTE_USER = "your-username"

# 使用 Git Bash 或 WSL 运行
bash ./deploy/deploy.sh
```

### 方式二：手动部署

#### Step 1: 本地编译
```powershell
# Windows PowerShell
cd F:\goproject\src\Bittap-watch
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w" -o ./deploy/validator ./cmd/validator
```

#### Step 2: 上传文件
```powershell
# 使用 scp 或 gcloud compute scp
scp ./deploy/validator root@35.243.114.46:/opt/latency-validator/
scp ./config.yaml root@35.243.114.46:/opt/latency-validator/
scp ./deploy/latency-validator.service root@35.243.114.46:/tmp/
```

#### Step 3: 远程配置
```bash
# SSH 到 VM
ssh root@35.243.114.46

# 创建目录
mkdir -p /opt/latency-validator/output
mkdir -p /opt/latency-validator/logs

# 设置权限
chmod +x /opt/latency-validator/validator

# 安装 systemd 服务
sudo mv /tmp/latency-validator.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable latency-validator
sudo systemctl start latency-validator
```

## 运维命令

### 服务管理
```bash
# 查看状态
sudo systemctl status latency-validator

# 启动/停止/重启
sudo systemctl start latency-validator
sudo systemctl stop latency-validator
sudo systemctl restart latency-validator

# 查看实时日志
journalctl -u latency-validator -f

# 查看最近 100 行日志
journalctl -u latency-validator -n 100
```

### 输出文件
```bash
# 查看输出目录
ls -la /opt/latency-validator/output/

# 实时查看 signals
tail -f /opt/latency-validator/output/signals.jsonl

# 实时查看 paper trades
tail -f /opt/latency-validator/output/paper_trades.jsonl

# 实时查看 metrics
tail -f /opt/latency-validator/output/metrics.jsonl
```

### 性能监控
```bash
# CPU 和内存
htop

# 网络连接（检查 WS 连接）
ss -tunp | grep validator

# 磁盘 IO
iotop
```

## 配置更新

修改配置后需要重启服务：
```bash
# 上传新配置
scp ./config.yaml root@35.243.114.46:/opt/latency-validator/

# 重启服务
ssh root@35.243.114.46 'sudo systemctl restart latency-validator'
```

## 故障排查

### 1. 服务无法启动
```bash
# 检查二进制是否可执行
file /opt/latency-validator/validator
chmod +x /opt/latency-validator/validator

# 手动运行查看错误
/opt/latency-validator/validator --config /opt/latency-validator/config.yaml
```

### 2. WS 连接失败
```bash
# 检查网络连通性
curl -I https://www.okx.com
curl -I https://fapi.binance.com
curl -I https://api.bittap.com

# 检查 DNS
nslookup ws.okx.com
nslookup fstream.binance.com
nslookup stream.bittap.com
```

### 3. 输出文件为空
```bash
# 检查目录权限
ls -la /opt/latency-validator/output/

# 检查磁盘空间
df -h
```
