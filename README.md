# edgeinfer-node-test

一个尽量小的 Wails v3 桌面应用底座。它不是管理后台，默认只提供后续业务应用普遍会复用的桌面能力。

## 内置能力

- Wails v3 + Go 后端，后端代码集中在 `server/`
- React + Bun + TanStack Router + shadcn/ui（Base UI）
- 单实例运行，重复启动会唤醒已有主窗口
- 开机自启动，可选择显示主窗口或后台运行（macOS SMAppService 除外）
- 系统托盘：打开主窗口、检查更新、退出
- 可配置关闭窗口时隐藏到托盘
- Gin 本地服务与 WebSocket `/ws` 示例，仅监听 `127.0.0.1:19876`
- WebSocket 仅允许 Wails/loopback 页面 Origin，并限制单条消息大小为 1 MiB
- WebView 内容区域默认禁止 HTML 拖拽和文本拖选；原生系统标题栏仍可正常移动窗口
- GitHub Actions 负责 Windows/Linux/macOS 构建，源码仓库可以保持私有
- 最终分发只走公开 CNB 仓库，不创建 GitHub Release
- 自定义 CNB updater provider，运行时不依赖 GitHub API / GitHub Releases
- 更新二进制使用 SHA-256 校验

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

## 项目级配置

统一入口是 `project/app.json`。复制底座开发新程序时优先只修改这里，然后执行：

```bash
go run ./cmd/buildmeta
```

当前示例：

```json
{
  "name": "edgeinfer-node-test",
  "binaryName": "edgeinfer-node-test",
  "identifier": "io.github.xu756.edge-desktop-base",
  "configDirName": "edgeinfer-node-test",
  "legacyConfigDirNames": ["edge-desktop-base"],
  "description": "Reusable Wails v3 desktop application foundation",
  "companyName": "xu756",
  "copyright": "(c) 2026, xu756",
  "updateRepositoryURL": "https://cnb.cool/xu756/public",
  "updateBranch": "main",
  "defaultAPIAddress": "127.0.0.1:19876",
  "devVersion": "0.1.0"
}
```

| 字段 | 用途 |
| --- | --- |
| `name` | 主窗口、托盘、界面及安装包显示名称 |
| `binaryName` | 可执行文件、`.app`、CNB Release Tag 和分发文件名前缀 |
| `identifier` | 单实例、自启动及平台应用标识 |
| `configDirName` | `~/.config` 下的配置目录名 |
| `legacyConfigDirNames` | 需要迁移的旧配置目录名 |
| `updateRepositoryURL` | 公开 CNB 分发仓库，例如 `https://cnb.cool/xu756/public` |
| `updateBranch` | CNB 分发仓库保存 manifest 的分支，默认 `main` |
| `companyName` / `description` / `copyright` | 平台资源与安装包元信息 |
| `defaultAPIAddress` | 本地 API 地址，只允许 loopback IP |
| `devVersion` | 本地开发版本；正式 Release 版本来自 Git Tag |

`buildmeta` 会同步 Wails 配置、Windows 版本资源、NSIS 元信息、macOS plist、Linux desktop/nfpm 元信息、网页标题等。根 `Taskfile.yml` 的 `APP_NAME` 直接读取 `project/app.json` 的 `binaryName`，不需要另外改名。

需要换品牌图标时替换 `build/appicon.png`。

## 发布架构

GitHub 源码仓库可以是私有仓库。GitHub Actions 仅负责构建，不承担最终分发：

```text
GitHub 私有源码仓库
        │
        │ push v0.2.0
        ▼
GitHub Actions
  ├─ Windows amd64
  ├─ Linux amd64
  └─ macOS arm64
        │
        ├─ 生成 SHA256SUMS
        │
        ▼
公开 CNB 分发仓库
https://cnb.cool/xu756/public
```

CNB 不需要 `.cnb.yml`，也不重复构建应用。

GitHub Actions 的临时 artifacts 只用于三个 runner 之间汇总，最终版本不会创建 GitHub Release。

### 版本命名

业务源码仓库仍使用普通语义化版本 Tag：

```text
v0.0.1
v0.0.2
v0.1.0
v0.2.0-beta.1
```

发布到共享 CNB 分发仓库时自动加入 `binaryName` 命名空间：

```text
edgeinfer-node-test-v0.0.1
edgeinfer-node-test-v0.0.2
edgeinfer-node-test-v0.1.0
edgeinfer-node-test-v0.2.0-beta.1
```

因此同一个公开仓库可以同时分发多个程序：

```text
edgeinfer-node-test-v0.0.2
another-desktop-app-v1.3.0
camera-client-v2.1.4
```

客户端不会使用 CNB 仓库级的 `latest Release` 判断版本，因为共享仓库中另一个程序发布后会改变全局 latest。

### CNB 仓库目录

CNB Git 仓库只保存很小的 JSON 更新元数据，不提交构建二进制，也不会把 GitHub 私有源码复制过去。

稳定版本示例：

```text
edgeinfer-node-test/
├── latest.json
├── v0.0.1/
│   └── manifest.json
├── v0.0.2/
│   └── manifest.json
└── v0.1.0/
    └── manifest.json
```

预发布版本还会维护：

```text
edgeinfer-node-test/prerelease.json
```

正式客户端默认只读取 `latest.json`，不会因为发布 `v0.2.0-beta.1` 自动升级到预发布版本。

`manifest.json` 示例：

```json
{
  "schemaVersion": 1,
  "app": "edgeinfer-node-test",
  "version": "0.0.2",
  "tag": "edgeinfer-node-test-v0.0.2",
  "channel": "stable",
  "publishedAt": "2026-09-08T12:00:00Z",
  "artifacts": [
    {
      "kind": "runtime",
      "platform": "windows",
      "arch": "amd64",
      "filename": "edgeinfer-node-test-windows-amd64.exe",
      "sha256": "...",
      "size": 12345678
    },
    {
      "kind": "installer",
      "platform": "windows",
      "arch": "amd64",
      "filename": "edgeinfer-node-test-windows-amd64-installer.exe",
      "sha256": "...",
      "size": 12345678
    }
  ]
}
```

每个历史版本目录中的 `manifest.json` 保留该版本记录；`latest.json` 是当前稳定版本 manifest 的副本。

### 二进制存储

大文件只作为对应 CNB Release 的附件，不提交到 Git 历史。

以 `edgeinfer-node-test-v0.0.2` 为例：

```text
edgeinfer-node-test-windows-amd64.exe
edgeinfer-node-test-windows-amd64-installer.exe
edgeinfer-node-test-linux-amd64
edgeinfer-node-test-linux-amd64.deb
edgeinfer-node-test-darwin-arm64.zip
edgeinfer-node-test-darwin-arm64.pkg
SHA256SUMS
```

文件名本身不重复携带版本号；版本由 namespaced CNB Release Tag 和 manifest 表达。

如果以后恢复 macOS Intel 构建，还会增加：

```text
edgeinfer-node-test-darwin-amd64.zip
edgeinfer-node-test-darwin-amd64.pkg
```

## 自动更新流程

运行时不再使用：

```go
github.com/wailsapp/wails/v3/pkg/updater/providers/github
```

项目自己的 `server/cnb_updater.go` 实现 Wails `updater.Provider`。

检查流程：

```text
当前程序版本 0.0.1
       │
       ▼
GET
https://cnb.cool/xu756/public/-/git/raw/main/edgeinfer-node-test/latest.json
       │
       ▼
校验：
- schemaVersion
- app == edgeinfer-node-test
- tag == edgeinfer-node-test-v<version>
- semver 新于当前版本
- platform / arch
- runtime 精确文件名
- SHA-256 格式
       │
       ▼
下载
https://cnb.cool/xu756/public/-/releases/download/
edgeinfer-node-test-v0.0.2/edgeinfer-node-test-windows-amd64.exe
       │
       ▼
Wails 校验 SHA-256
       │
       ▼
Wails 原子替换 + 重启
```

一个程序永远只读取自己的：

```text
<binaryName>/latest.json
```

并要求：

```text
manifest.app == <binaryName>
manifest.tag == <binaryName>-v<manifest.version>
```

所以共享分发仓库中其他项目的 Release 不会参与当前程序的版本判断。

## DEB / PKG 更新策略

系统安装位置继续交给系统安装包管理，不做应用内文件覆盖：

- Linux `/usr/bin/<binaryName>`：下载新版 `.deb` 安装升级
- macOS `/Applications/<binaryName>.app`：下载新版 `.pkg` 安装升级

点击“检查更新”时会读取该程序自己的 CNB `latest.json`，然后打开对应的 CNB Release 页面。

用户目录中的免安装 Windows EXE、Linux binary、macOS `.app` 仍使用应用内自动更新。

## GitHub Actions 配置

在私有源码仓库：

```text
Settings
→ Secrets and variables
→ Actions
```

建议配置：

### Repository variables

```text
CNB_REPO_URL=https://cnb.cool/xu756/public
CNB_TARGET_BRANCH=main
CNB_USERNAME=cnb
```

其中 `CNB_REPO_URL` / `CNB_TARGET_BRANCH` 如果配置，必须与 `project/app.json` 保持一致，脚本会主动校验，防止把私有项目产物误传到错误仓库。

`CNB_USERNAME` 可以不配置，默认就是：

```text
cnb
```

### Repository secret

```text
CNB_TOKEN=<CNB Access Token>
```

这里使用 CNB **访问令牌**，不是 CNB 登录账号密码，也不要使用只读部署令牌。访问令牌需要对目标公开分发仓库具备 Git 写入和 Release 写入权限。

`CNB_TOKEN` 仅存在 GitHub Actions Secrets 中，不会编译进客户端，也不会写入公开 manifest。

发布脚本使用：

- CNB OpenAPI 创建/复用 namespaced Release
- CNB OpenAPI 上传 Release 附件
- HTTPS Git + Access Token 更新 `<binaryName>/...` JSON manifest

manifest 始终在全部 Release 附件上传成功后才提交，避免客户端看到一个指向未完整上传版本的 `latest.json`。

## 发布一个版本

始终从最新 `main` 创建 Tag：

```bash
git checkout main
git pull --ff-only origin main

git tag v0.2.0
git push origin v0.2.0
```

最终 CNB 中会得到：

```text
Release Tag:
edgeinfer-node-test-v0.2.0

Git metadata:
edgeinfer-node-test/v0.2.0/manifest.json
edgeinfer-node-test/latest.json
```

如果发布：

```bash
git tag v0.3.0-beta.1
git push origin v0.3.0-beta.1
```

则得到：

```text
Release Tag:
edgeinfer-node-test-v0.3.0-beta.1

Git metadata:
edgeinfer-node-test/v0.3.0-beta.1/manifest.json
edgeinfer-node-test/prerelease.json
```

不会覆盖稳定版 `latest.json`。

## 用户配置与自启动

首次运行会创建 `settings.json`，配置独立于程序安装位置，自更新不会替换它。所有桌面平台统一使用用户主目录下的 `.config`，目录名由项目配置中的 `configDirName` 决定：

- Windows：`%USERPROFILE%\.config\edgeinfer-node-test\settings.json`
- macOS / Linux：`~/.config/edgeinfer-node-test/settings.json`

这里明确使用用户主目录，不跟随 `XDG_CONFIG_HOME`。新文件不存在时，会从平台原 `os.UserConfigDir()` 位置以及 `~/.config` 下的旧目录迁移；旧目录名来自 `legacyConfigDirNames`。迁移保留原文件、不覆盖已有的新文件。

默认内容：

```json
{
  "autoCheckUpdates": true,
  "updateIntervalHours": 6,
  "closeToTray": true,
  "autoStartShowWindow": false
}
```

`autoCheckUpdates` 控制定时检查，关闭后仍可手动检查；`updateIntervalHours` 允许 1–168 小时。

“开机自动启动”以操作系统注册状态为准，不在 JSON 中重复保存启用状态。“自启动时显示主窗口”保存在 JSON 中，可在开启自启动前设置。

**macOS 限制：** 当前 Wails beta.17 在 macOS 13+ 的打包 `.app` 中使用 SMAppService，忽略自定义参数；该模式下窗口开关会禁用并提示，由系统决定启动窗口行为。

更新窗口模板位于 `server/updater-window.html`，基于 Wails beta.17 的 MIT 模板做中文化和样式定制。升级 Wails 时需核对 updater Provider 接口、事件协议和 runtime-ready 握手。

## Windows 安装包

安装 NSIS 并将 `makensis` 加入 PATH 后运行：

```bash
wails3 task windows:package ARCH=amd64 INSTALL_SCOPE=user
```

默认即为 `user`，安装目录为 `%LocalAppData%\Programs\<binaryName>`。

生成：

- `bin/<binaryName>.exe`：运行文件，也是便携版自动更新 payload
- `bin/<binaryName>-amd64-installer.exe`：首次安装 NSIS 包

Actions 会重命名成固定 CNB 分发文件名并计算校验和。

## Ubuntu DEB 与 macOS PKG

Ubuntu 本地构建：

```bash
wails3 task linux:create:deb ARCH=amd64
sudo apt install ./bin/edgeinfer-node-test.deb
```

DEB 面向 Ubuntu 24.04+，程序安装到 `/usr/bin/<binaryName>`，配置仍保存在用户目录。

macOS 本地构建：

```bash
wails3 task darwin:package:pkg ARCH=arm64
# Intel Mac 如需本地构建可使用 ARCH=amd64
```

生成 `bin/<binaryName>-<arch>.pkg`，安装到 `/Applications/<binaryName>.app`。

生产公开分发前仍建议补 Windows Authenticode 与 macOS Developer ID / notarization。SHA-256 能校验下载完整性，但不能代替平台代码签名。
