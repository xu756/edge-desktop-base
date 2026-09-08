# edgeinfer-node-test

一个尽量小的 Wails v3 桌面应用底座。它不是管理后台，默认只提供后续业务应用普遍会复用的桌面能力。

## 内置能力

- Wails v3 + Go 后端，后端代码集中在 `server/`
- React + Bun + TanStack Router + shadcn/ui（Base UI）
- 单实例运行，重复启动会唤醒已有主窗口
- 开机自启动，可选择显示主窗口或后台运行（macOS SMAppService 除外）
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
edgeinfer-node-test-v0.2.0-windows-amd64-installer.exe
edgeinfer-node-test-v0.2.0-linux-amd64
edgeinfer-node-test-v0.2.0-darwin-arm64.zip
edgeinfer-node-test-v0.2.0-darwin-amd64.zip
SHA256SUMS
```

更新器严格匹配 `<binaryName>-v<版本>-<平台>-<架构>` 形式的运行文件（Windows 为 `.exe`，macOS 为 `.zip`，Linux 无扩展名），排除安装包、签名等附件，并使用同一 Release 中的 `SHA256SUMS` 校验下载内容。

当前发布目标：Windows amd64（安装包和更新 EXE）、Linux amd64、macOS arm64、macOS amd64。

生产发布前建议补 Windows Authenticode 与 macOS Developer ID / notarization。SHA-256 能校验文件完整性，但代码签名仍然是正式分发时需要补齐的一层。

`build/appicon.png` 继续作为应用和托盘图标源，需要换品牌时替换这个源文件即可。

## 用户配置与自启动

首次运行会创建 `settings.json`，配置独立于程序安装位置，自更新不会替换它。所有桌面平台统一使用用户主目录下的 `.config`，目录名由项目配置中的 `configDirName` 决定：

- Windows：`%USERPROFILE%\.config\edgeinfer-node-test\settings.json`
- macOS / Linux：`~/.config/edgeinfer-node-test/settings.json`

这里明确使用用户主目录，不跟随 `XDG_CONFIG_HOME`。新文件不存在时，会从平台原 `os.UserConfigDir()` 位置以及 `~/.config` 下的旧目录迁移；旧目录名来自 `legacyConfigDirNames`。迁移保留原文件、不覆盖已有的新文件；损坏的旧配置也保留并报告错误。

界面展示实际配置路径。默认内容：

```json
{
  "autoCheckUpdates": true,
  "updateIntervalHours": 6,
  "closeToTray": true,
  "autoStartShowWindow": false
}
```

`autoCheckUpdates` 控制定时检查，关闭后仍可手动检查；`updateIntervalHours` 允许 1–168 小时。手动编辑配置后重启生效。旧配置缺失的新字段采用默认值；无法读取或格式错误时使用默认设置，界面提示错误且禁止覆盖原文件，修复文件后重启。

“开机自动启动”以操作系统注册状态为准，不在 JSON 中重复保存启用状态。“自启动时显示主窗口”保存在 JSON 中，可在开启自启动前设置；已开启时修改会同步更新注册参数。Windows/Linux 及 macOS LaunchAgent 使用 `--autostart`，后台模式额外携带 `--hidden`；普通手动启动仍显示窗口。单独使用 `--hidden` 也保持兼容。更改可执行文件位置后应重新开启自启动以更新注册路径。

**macOS 限制：** 当前 Wails beta.17 在 macOS 13+ 的打包 `.app` 中使用 SMAppService，忽略自定义参数；该模式下窗口开关会禁用并提示，由系统决定启动窗口行为。

更新窗口模板位于 `server/updater-window.html`，基于 Wails beta.17 的 MIT 模板做中文化和样式定制。修改品牌、颜色或文案可直接编辑该文件；升级 Wails 时需核对事件协议和 runtime-ready 握手。Actions 仍仅由版本 tag 触发，Windows 构建额外生成用户级 NSIS 安装包。


## 项目级配置（开发者修改）

统一入口是 `project/app.json`。它随源码构建进程序，不是用户运行时的 `settings.json`，不应放密钥。

| 字段 | 用途 |
| --- | --- |
| `name` | 主窗口、托盘、界面及安装包显示名称 |
| `binaryName` | 可执行文件、`.app` 和 Release 文件名前缀，使用小写字母、数字、连字符 |
| `identifier` | 单实例、自启动及平台应用标识 |
| `configDirName` | `~/.config` 下的配置目录名 |
| `legacyConfigDirNames` | 需要迁移的旧配置目录名，按顺序查找 |
| `updateRepository` | GitHub 更新仓库，格式为 `owner/repo` |
| `companyName` / `description` / `copyright` | 平台资源与安装包元信息 |
| `defaultAPIAddress` | 本地 API 地址，只允许 loopback IP |
| `devVersion` | 本地构建版本；Release 版本仍来自 tag |

修改后可以执行：

```bash
wails3 task configure
wails3 task dev
```

正常桌面构建会自动执行同步。`configure` 更新 Wails 配置、Windows 版本资源和 NSIS 元信息、macOS plist、Linux desktop/nfpm 元信息及网页标题；生成文件无需手工改名。本次统一范围为桌面底座，未接入的 iOS/Android/MSIX 模板不在此流程内。不要用 `wails3 update build-assets` 覆盖定制的 Taskfile/NSIS 脚本；需要升级框架模板时应逐项合并。

仅修改显示名称时，保留 `binaryName`、`identifier`、`configDirName`，这样更新路径、启动项和配置保持连续。复制底座开发独立产品时再更改这些字段。更换 `binaryName` 或更新仓库会影响已发布客户端的资产匹配，不能仅把新 Release 文件改名后就期望旧客户端自动迁移。独立产品不需要继承底座设置时，将 `legacyConfigDirNames` 设为 `[]`。

## Windows 安装包

安装 NSIS 并将 `makensis` 加入 PATH 后运行：

```bash
wails3 task windows:package ARCH=amd64 INSTALL_SCOPE=user
```

默认即为 `user`，安装目录为 `%LocalAppData%\Programs\<binaryName>`。安装包创建开始菜单、桌面快捷方式和卸载入口；卸载保留 `~/.config` 中的用户设置，并清理当前用户的应用自启动注册项。

生成两个文件：

- `bin/<binaryName>.exe`：应用本体，也是自动更新使用的文件。
- `bin/<binaryName>-amd64-installer.exe`：首次安装用的 NSIS 安装包。

Actions 自动安装 NSIS，并把两个文件一起发布和计算校验和。后续原地更新无需重新运行安装包，要求安装目录对当前用户可写。自行改为机器级安装或选择受保护目录可能导致更新权限不足。原地更新不会重新执行安装脚本，也不会自动刷新 Windows 卸载列表的版本号。
