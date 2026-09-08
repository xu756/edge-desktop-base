# Edge Desktop Base

一个尽量小的 Wails v3 桌面应用底座。它不是管理后台，默认只提供后续业务应用普遍会复用的桌面能力。

## 内置能力

- Wails v3 + Go 后端，后端代码集中在 `server/`
- React + Bun + TanStack Router + shadcn/ui（Base UI）
- 单实例运行，重复启动会唤醒已有主窗口
- 开机自启动，启用后携带 `--hidden` 后台启动
- 系统托盘：打开主窗口、检查更新、退出
- 可配置关闭窗口时隐藏到托盘
- GitHub Releases 自动更新，每 6 小时检查一次
- Release 使用 `SHA256SUMS` 校验下载文件
- 前端通过 Wails bindings 直接调用 Go Service
- Gin 本地服务与 WebSocket `/ws` 示例，仅监听 `127.0.0.1:19876`
- WebSocket 仅允许 Wails/loopback 页面 Origin，并限制单条消息大小为 1 MiB
- WebView 内容区域默认禁止 HTML 拖拽和文本拖选；原生系统标题栏仍可正常移动窗口
- GitHub Actions 只在发布 `v*` tag 时构建 Windows/Linux/macOS 并创建 Release

## 开发

```bash
go mod tidy
cd frontend && bun install && cd ..
wails3 task dev
```

Wails 会在开发/构建时自动生成 `frontend/bindings`，前端的服务调用来自 `server.DesktopService`。

## WebSocket

```text
ws://127.0.0.1:19876/ws
```

连接后服务端先发送 `runtime.ready`。前端发送任意 JSON 后，当前示例会以 `echo` 消息回传，后续业务可以直接替换为自己的事件协议。

健康检查：

```text
GET http://127.0.0.1:19876/health
```

## 发布与自动更新

Actions 不响应 `main`/功能分支 push，也不提供手动构建入口。只有推送符合语义化版本格式的 `v*` tag 才会执行发布构建：

```bash
git tag v0.2.0
git push origin v0.2.0
```

也支持预发布 tag，例如：

```bash
git tag v0.3.0-beta.1
git push origin v0.3.0-beta.1
```

构建时会直接读取 tag：`v0.2.0` 会注入为应用版本 `0.2.0`，同时同步到 Wails `build/config.yml`。运行时版本不带前导 `v`，与 Wails GitHub updater 的版本规则一致。

Release 资产包含 tag、OS 和 Arch，例如：

```text
edgeinfer-node-test-v0.2.0-windows-amd64.exe
edgeinfer-node-test-v0.2.0-linux-amd64
edgeinfer-node-test-v0.2.0-darwin-arm64.zip
edgeinfer-node-test-v0.2.0-darwin-amd64.zip
SHA256SUMS
```

Wails GitHub updater 根据 `windows|linux|darwin` 和 `amd64|arm64` 自动选择当前平台的文件，并使用同一 Release 中的 `SHA256SUMS` 校验下载内容。

当前发布目标：Windows amd64、Linux amd64、macOS arm64、macOS amd64。

生产发布前建议补 Windows Authenticode 与 macOS Developer ID / notarization。SHA-256 能校验文件完整性，但代码签名仍然是正式分发时需要补齐的一层。

`build/appicon.png` 继续作为应用和托盘图标源，需要换品牌时替换这个源文件即可。
