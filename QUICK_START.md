# 延迟套利验证器 - 快速参考

## 📊 访问 Dashboard

直接在浏览器打开：

```
http://bittapwatch.duckdns.org
```

无需任何配置，任何设备、任何网络都可以访问。

---

## 🔧 在家里电脑管理服务器

### 前置条件

1. 安装 [Google Cloud SDK](https://cloud.google.com/sdk/docs/install)
2. 登录 Google 账户：
   ```powershell
   gcloud auth login
   # 选择 hkhzxudeyuan@gmail.com 登录
   ```
3. 设置项目：
   ```powershell
   gcloud config set project swift-apogee-471008-d1
   ```

### 常用命令

```powershell
# 进入项目目录
cd F:\goproject\src\Bittap-watch\deploy

# 查看帮助
.\deploy.ps1

# 查看服务状态
.\deploy.ps1 status

# 查看实时日志
.\deploy.ps1 logs

# 重启服务
.\deploy.ps1 restart

# SSH 登录服务器
.\deploy.ps1 ssh

# 完整部署（修改代码后）
.\deploy.ps1 deploy

# 快速部署（只更新二进制）
.\deploy.ps1 quick

# 下载数据文件到本地
.\deploy.ps1 download
```

---

## 📡 API 端点

| 端点 | 说明 |
|------|------|
| `/api/status` | 实时状态 |
| `/api/summary` | 汇总统计 |
| `/api/metrics` | 历史 metrics |
| `/api/signals` | 信号记录 |
| `/api/trades` | 影子成交记录 |

示例：
```
http://bittapwatch.duckdns.org/api/summary
```

---

## 🖥️ 服务器信息

| 项目 | 值 |
|------|-----|
| GCP 实例 | instance-20251017-060424 |
| 区域 | asia-northeast1-b (东京) |
| 外部 IP | 35.243.114.46 |
| 域名 | bittapwatch.duckdns.org |
| 项目 ID | swift-apogee-471008-d1 |
| Google 账户 | hkhzxudeyuan@gmail.com |

---

## 🔑 DuckDNS 信息（域名管理）

- 域名: `bittapwatch.duckdns.org`
- Token: `c20291f2-abdf-4952-a061-4ce4a4e83acd`
- 管理页面: https://www.duckdns.org

如果 IP 变化，更新 DNS：
```
https://www.duckdns.org/update?domains=bittapwatch&token=c20291f2-abdf-4952-a061-4ce4a4e83acd&ip=新IP地址
```

---

## 📁 服务器目录结构

```
/opt/latency-validator/
├── validator              # Go 二进制
├── config.yaml            # 配置文件
├── output/
│   ├── metrics.jsonl      # 系统指标
│   ├── signals.jsonl      # 信号记录
│   └── paper_trades.jsonl # 影子成交
└── dashboard/
    ├── api.py             # Flask API
    └── static/
        └── index.html     # 前端页面
```

---

## 🚨 故障排查

### 服务无法访问
```powershell
.\deploy.ps1 status
.\deploy.ps1 logs
```

### 重启所有服务
```powershell
.\deploy.ps1 ssh
# 然后在服务器上：
sudo systemctl restart latency-validator
sudo systemctl restart dashboard
sudo systemctl restart nginx
```

### 查看具体错误
```powershell
.\deploy.ps1 ssh
# 然后在服务器上：
sudo journalctl -u latency-validator -n 100
sudo journalctl -u dashboard -n 100
```
