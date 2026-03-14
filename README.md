# Mihomo-Tool

一个基于 Mihomo 核心的工具，专为简化订阅管理和中转链（Relay Chain）配置而设计。

---

## 🌟 核心特性

- **🚀 极简订阅管理**：支持从 Clash 订阅链接自动拉取节点信息，并支持设置自定义 User-Agent。
- **📊 智能压力测试**：
    - **延迟测试**：自动对所有订阅节点进行毫秒级延迟测试。
    - **全自动下载测速**：自动选取延迟最低的前 $N$ 个节点（数量可调）进行实际下载速度测试。
    - **自动优选**：根据测速结果自动选择最优节点作为代理前置。
- **🔗 一键生成中转链**：
    - 将本地最优节点与指定的目标落地节点（Landing Proxy）自动组合成中转链。
    - 利用现代 Mihomo 的 `dialer-proxy` 链式调用技术，实现极致稳定的转发。
- **🛡️ 容错备份 (Fallback)**：支持 Fallback 模式，当主节点失效时自动切换至备用节点。
- **🎨 极美 Web UI**：
    - **毛玻璃效果**：内置简洁现代的 Web 控制面板（默认端口 `58888`）。
    - **实时监控**：提供实时后端日志输出与测速进度条展示。
    - **多端适配**：支持响应式布局，无论宽屏还是移动端都有极佳表现。
- **📦 内核自管理**：
    - 支持在 UI 中一键下载、更新和安装 Mihomo 内核。
    - 自动清理占用端口，确保服务稳定启动。
- **🌍 跨平台支持**：完美支持 Windows、Linux (systemd)、OpenWrt (procd) 和 macOS。

## 🚀 快速开始

### Linux / OpenWrt (一键管理)

在终端运行以下命令进行安装：

```bash
curl -sSL https://raw.githubusercontent.com/obkj/mihomo-tool/main/mihomo-tool.sh | sudo bash -s install
```

**卸载**：

```bash
curl -sSL https://raw.githubusercontent.com/obkj/mihomo-tool/main/mihomo-tool.sh | sudo bash -s uninstall
```

### Windows

1. 从 [Releases](https://github.com/obkj/mihomo-tool/releases) 页面下载 `mihomo-tool-windows-amd64.zip`。
2. 解压并双击运行 `mihomo-tool.exe`。

### 使用说明

1. **访问控制台**：在浏览器打开 `http://<your-ip>:58888`。
2. **一键安装内核**：首次使用点击界面上的 "Install" 按钮，程序将自动下载并安装最新的 Mihomo 内核。
3. **配置与启动**：
    - 在“设置”页面填入您的订阅链接。
    - 设置合适的更新间隔（如 60 分钟）。
    - 填入您的目标落地节点链接（Landing Proxy）。
    - 点击“更新订阅” -> “保存并应用”。
4. **开始冲浪**：点击“启动”运行核心。默认 HTTP 代理端口为 `7890`，SOCKS5 为 `7891`。

## 🛠️ 技术架构

- **后端 (Backend)**: [Go](https://go.dev/) - 负责高性能并发逻辑、进程管理、内核下载及 HTTP API。
- **前端 (Frontend)**: [Vanilla JS](https://developer.mozilla.org/en-US/docs/Web/JavaScript) + [CSS3](https://developer.mozilla.org/en-US/docs/Web/CSS) - 现代毛玻璃设计风格，无需重度框架，轻量且快速。
- **核心 (Core)**: [Mihomo (Meta)](https://github.com/MetaCubeX/mihomo) - 提供最底层的网络流量转发支持。

## 📄 开源协议

本项目采用 [MIT License](LICENSE) 开源。欢迎大家提交 Issue 和 PR！
