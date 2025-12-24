# 部署脚本：将 latency-arbitrage-validator 部署到 GCP Tokyo VM
# 目标 IP: 35.243.114.46
# PowerShell 版本

param(
    [string]$RemoteUser = "root",
    [string]$RemoteHost = "35.243.114.46",
    [string]$RemoteDir = "/opt/latency-validator"
)

$ErrorActionPreference = "Stop"
$ProjectDir = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)

Write-Host "=== 部署 latency-arbitrage-validator ===" -ForegroundColor Cyan
Write-Host "目标: ${RemoteUser}@${RemoteHost}:${RemoteDir}"
Write-Host "本地项目: ${ProjectDir}"

# ==== Step 1: 交叉编译 Linux amd64 二进制 ====
Write-Host ""
Write-Host "[1/4] 交叉编译 Linux amd64 二进制..." -ForegroundColor Yellow
Set-Location $ProjectDir

$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

$OutputBinary = Join-Path $ProjectDir "deploy\validator"
go build -ldflags="-s -w" -o $OutputBinary ./cmd/validator

if (-not (Test-Path $OutputBinary)) {
    Write-Host "编译失败！" -ForegroundColor Red
    exit 1
}

$FileSize = (Get-Item $OutputBinary).Length / 1MB
Write-Host "编译完成: $OutputBinary ($([math]::Round($FileSize, 2)) MB)" -ForegroundColor Green

# 清理环境变量
Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue

# ==== Step 2: 创建远程目录 ====
Write-Host ""
Write-Host "[2/4] 创建远程目录..." -ForegroundColor Yellow
ssh "${RemoteUser}@${RemoteHost}" "mkdir -p ${RemoteDir}/output && mkdir -p ${RemoteDir}/logs"

# ==== Step 3: 上传文件 ====
Write-Host ""
Write-Host "[3/4] 上传文件..." -ForegroundColor Yellow
scp "$OutputBinary" "${RemoteUser}@${RemoteHost}:${RemoteDir}/validator"
scp "$ProjectDir\config.yaml" "${RemoteUser}@${RemoteHost}:${RemoteDir}/config.yaml"
scp "$ProjectDir\deploy\latency-validator.service" "${RemoteUser}@${RemoteHost}:/tmp/latency-validator.service"

# ==== Step 4: 配置 systemd 并启动服务 ====
Write-Host ""
Write-Host "[4/4] 配置 systemd 服务..." -ForegroundColor Yellow

$RemoteCommands = @"
sudo mv /tmp/latency-validator.service /etc/systemd/system/latency-validator.service
sudo chmod 644 /etc/systemd/system/latency-validator.service
sudo chmod +x ${RemoteDir}/validator
sudo systemctl daemon-reload
sudo systemctl enable latency-validator
sudo systemctl restart latency-validator
sleep 2
sudo systemctl status latency-validator --no-pager || true
"@

ssh "${RemoteUser}@${RemoteHost}" $RemoteCommands

Write-Host ""
Write-Host "=== 部署完成 ===" -ForegroundColor Green
Write-Host "查看日志: ssh ${RemoteUser}@${RemoteHost} 'journalctl -u latency-validator -f'" -ForegroundColor Cyan
Write-Host "查看输出: ssh ${RemoteUser}@${RemoteHost} 'ls -la ${RemoteDir}/output/'" -ForegroundColor Cyan
