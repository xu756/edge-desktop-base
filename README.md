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
- GitHub Actions 构建 Windows/Linux/macOS，并在 tag 发布时创建 Release

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

手动运行 Actions 可以只验证构建。正式发布使用 tag：

```bash
git tag v0.2.0
git push origin v0.2.0
```

Actions 会生成带 OS/Arch 的更新资产以及 `SHA256SUMS`。Wails GitHub updater 根据 `windows|linux|darwin` 和 `amd64|arm64` 自动选择当前平台的文件。

当前发布目标：Windows amd64、Linux amd64、macOS arm64、macOS amd64。

生产发布前建议再补 Windows Authenticode 与 macOS Developer ID / notarization。

`build/appicon.png` 继续作为应用和托盘图标源，需要换品牌时替换这个源文件即可。
