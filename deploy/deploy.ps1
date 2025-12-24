# ============================================================
# 延迟套利验证器 - 部署与运维脚本
# ============================================================
# 
# 【配置信息】
#   GCP 实例: instance-20251017-060424
#   区域: asia-northeast1-b
#   外部 IP: 35.243.114.46
#   域名: bittapwatch.duckdns.org
#   Dashboard: http://bittapwatch.duckdns.org
#
# 【使用方法】
#   .\deploy.ps1 deploy      # 完整部署（编译+上传+重启）
#   .\deploy.ps1 quick       # 快速部署（仅上传二进制+重启）
#   .\deploy.ps1 status      # 查看服务状态
#   .\deploy.ps1 logs        # 查看实时日志
#   .\deploy.ps1 restart     # 重启服务
#   .\deploy.ps1 ssh         # SSH 登录服务器
#   .\deploy.ps1 download    # 下载输出文件到本地
#
# ============================================================

param(
    [Parameter(Position=0)]
    [ValidateSet("deploy", "quick", "status", "logs", "restart", "ssh", "download", "help")]
    [string]$Action = "help"
)

# 配置
$Instance = "instance-20251017-060424"
$Zone = "asia-northeast1-b"
$RemoteDir = "/opt/latency-validator"
$ProjectDir = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)

function Show-Help {
    Write-Host @"
============================================================
延迟套利验证器 - 部署与运维脚本
============================================================

【常用命令】
  .\deploy.ps1 deploy      完整部署（编译+上传+重启）
  .\deploy.ps1 quick       快速部署（仅上传二进制+重启）
  .\deploy.ps1 status      查看服务状态
  .\deploy.ps1 logs        查看实时日志（Ctrl+C 退出）
  .\deploy.ps1 restart     重启服务
  .\deploy.ps1 ssh         SSH 登录服务器
  .\deploy.ps1 download    下载输出文件到本地

【访问地址】
  Dashboard: http://bittapwatch.duckdns.org
  API:       http://bittapwatch.duckdns.org/api/status

【服务器信息】
  实例: $Instance
  区域: $Zone
  IP:   35.243.114.46
============================================================
"@ -ForegroundColor Cyan
}

function Build-Binary {
    Write-Host "[BUILD] 交叉编译 Linux amd64..." -ForegroundColor Yellow
    Set-Location $ProjectDir
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"
    
    $OutputBinary = Join-Path $ProjectDir "deploy\validator"
    go build -ldflags="-s -w" -o $OutputBinary ./cmd/validator
    
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    
    if (-not (Test-Path $OutputBinary)) {
        Write-Host "[ERROR] 编译失败！" -ForegroundColor Red
        exit 1
    }
    $FileSize = [math]::Round((Get-Item $OutputBinary).Length / 1MB, 2)
    Write-Host "[BUILD] 完成: $OutputBinary ($FileSize MB)" -ForegroundColor Green
}

function Deploy-Full {
    Write-Host "=== 完整部署 ===" -ForegroundColor Cyan
    Build-Binary
    
    Write-Host "[UPLOAD] 上传文件..." -ForegroundColor Yellow
    gcloud compute scp "$ProjectDir\deploy\validator" "${Instance}:/tmp/validator" --zone=$Zone
    gcloud compute scp "$ProjectDir\config.yaml" "${Instance}:/tmp/config.yaml" --zone=$Zone
    
    Write-Host "[DEPLOY] 部署并重启..." -ForegroundColor Yellow
    gcloud compute ssh $Instance --zone=$Zone --command="sudo mv /tmp/validator /opt/latency-validator/ && sudo mv /tmp/config.yaml /opt/latency-validator/ && sudo chmod +x /opt/latency-validator/validator && sudo systemctl restart latency-validator && sleep 2 && sudo systemctl status latency-validator --no-pager"
    
    Write-Host ""
    Write-Host "=== 部署完成 ===" -ForegroundColor Green
    Write-Host "Dashboard: http://bittapwatch.duckdns.org" -ForegroundColor Cyan
}

function Deploy-Quick {
    Write-Host "=== 快速部署 ===" -ForegroundColor Cyan
    Build-Binary
    
    Write-Host "[UPLOAD] 上传二进制..." -ForegroundColor Yellow
    gcloud compute scp "$ProjectDir\deploy\validator" "${Instance}:/tmp/validator" --zone=$Zone
    
    Write-Host "[DEPLOY] 重启服务..." -ForegroundColor Yellow
    gcloud compute ssh $Instance --zone=$Zone --command="sudo mv /tmp/validator /opt/latency-validator/ && sudo chmod +x /opt/latency-validator/validator && sudo systemctl restart latency-validator && sleep 2 && sudo systemctl status latency-validator --no-pager"
    
    Write-Host ""
    Write-Host "=== 部署完成 ===" -ForegroundColor Green
}

function Show-Status {
    Write-Host "=== 服务状态 ===" -ForegroundColor Cyan
    gcloud compute ssh $Instance --zone=$Zone --command="sudo systemctl status latency-validator --no-pager && echo '' && echo '=== 输出文件 ===' && ls -lh /opt/latency-validator/output/ && echo '' && echo '=== 最新 metrics ===' && tail -1 /opt/latency-validator/output/metrics.jsonl | python3 -c 'import sys,json; d=json.load(sys.stdin); print(f\"Trades: OKX={d[\"\"ev_okx\"\"][\"\"Count\"\"]}, Binance={d[\"\"ev_binance\"\"][\"\"Count\"\"]}\")'  2>/dev/null || echo 'No metrics yet'"
}

function Show-Logs {
    Write-Host "=== 实时日志 (Ctrl+C 退出) ===" -ForegroundColor Cyan
    gcloud compute ssh $Instance --zone=$Zone --command="sudo journalctl -u latency-validator -f"
}

function Restart-Service {
    Write-Host "=== 重启服务 ===" -ForegroundColor Cyan
    gcloud compute ssh $Instance --zone=$Zone --command="sudo systemctl restart latency-validator && sleep 2 && sudo systemctl status latency-validator --no-pager"
}

function Start-SSH {
    Write-Host "=== SSH 登录 ===" -ForegroundColor Cyan
    gcloud compute ssh $Instance --zone=$Zone
}

function Download-Output {
    Write-Host "=== 下载输出文件 ===" -ForegroundColor Cyan
    $LocalOutput = Join-Path $ProjectDir "output_download"
    New-Item -ItemType Directory -Force -Path $LocalOutput | Out-Null
    gcloud compute scp "${Instance}:/opt/latency-validator/output/*.jsonl" $LocalOutput --zone=$Zone
    Write-Host "下载完成: $LocalOutput" -ForegroundColor Green
}

# 执行
switch ($Action) {
    "deploy"   { Deploy-Full }
    "quick"    { Deploy-Quick }
    "status"   { Show-Status }
    "logs"     { Show-Logs }
    "restart"  { Restart-Service }
    "ssh"      { Start-SSH }
    "download" { Download-Output }
    default    { Show-Help }
}
