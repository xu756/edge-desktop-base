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
- GitHub Actions 只在发布 `v*` tag 时构建 Windows/Linux/macOS 并创建 GitHub Release
- 可选将 GitHub Actions 已构建的同一批 Release 产物直接同步到 CNB Release，CNB 不再重复构建

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

Actions 不响应 `main`/功能分支 push，也不提供手动构建入口。只有推送符合语义化版本格式的 `v*` tag 才会执行发布构建。建议始终在最新 `main` 提交上创建发布 tag：

```bash
git checkout main
git pull --ff-only origin main
git tag v0.2.0
git push origin v0.2.0
```

也支持预发布 tag，例如：

```bash
git tag v0.3.0-beta.1
git push origin v0.3.0-beta.1
```

构建时会直接读取 tag：`v0.2.0` 会注入为应用版本 `0.2.0`，同时同步到 Wails `build/config.yml`、Windows 版本资源、NSIS 元信息、macOS plist 和 Linux nfpm 元信息。运行时版本不带前导 `v`，与 Wails GitHub updater 的版本规则一致。

版本号只由 Git Tag、Release 和应用/安装包元数据表达，发布文件名保持固定，不重复包含版本号。这样不同版本的下载路径结构、自动化脚本和 updater asset matcher 都保持稳定。

当前 GitHub Actions 自动发布目标：

- Windows amd64：应用更新 EXE + 用户级 NSIS installer
- Linux amd64：应用更新二进制 + DEB installer
- macOS arm64：应用更新 ZIP + PKG installer
- macOS amd64：Task 已支持，但 Actions matrix 当前暂时注释，不自动发布

每个正式 Release 使用固定资产名：

```text
edgeinfer-node-test-windows-amd64.exe
edgeinfer-node-test-windows-amd64-installer.exe
edgeinfer-node-test-linux-amd64
edgeinfer-node-test-linux-amd64.deb
edgeinfer-node-test-darwin-arm64.zip
edgeinfer-node-test-darwin-arm64.pkg
SHA256SUMS
```

如果以后重新启用 macOS Intel matrix，还会额外发布：

```text
edgeinfer-node-test-darwin-amd64.zip
edgeinfer-node-test-darwin-amd64.pkg
```

### 同步发布到 CNB

CNB 不再使用 `.cnb.yml` 构建应用。GitHub Actions 在三个桌面平台全部构建完成后，只生成一次 `SHA256SUMS`，先发布 GitHub Release，再把 `release/` 目录中的**同一批文件**同步到同 Tag 的 CNB Release。CNB 因此不需要 Runner，也不会发生 GitHub/CNB 两边分别编译导致的产物差异。

如需开启同步，在 GitHub 仓库 `Settings → Secrets and variables → Actions` 配置：

- Repository variable `CNB_REPO_SLUG`：CNB 完整仓库路径，例如 `xu756/edgeinfer-node-test`。
- Repository secret `CNB_TOKEN`：CNB **访问令牌**，需要目标仓库 Release 读写权限 `repo-release:rw`；不要使用只读部署令牌。
- Repository variable `CNB_TARGET_BRANCH`：可选，CNB Release 对应的目标分支；未设置时默认 `main`。

未配置 `CNB_REPO_SLUG` 时，GitHub Actions 会直接跳过 CNB 同步，GitHub Release 正常发布。配置了 `CNB_REPO_SLUG` 后，如果 `CNB_TOKEN` 缺失、权限不足或 CNB API 上传失败，Release job 会失败并明确暴露同步错误。

同步逻辑位于 `scripts/publish-cnb-release.py`：CNB Release 不存在时创建，已存在时直接复用，并覆盖上传同名附件；正式版本和 `v1.2.3-beta.1` 这类 prerelease 都会按 Git Tag 同步。附件通过 CNB OpenAPI 的预签名上传地址直接上传，不经过 CNB 构建流水线。

更新器严格精确匹配 `<binaryName>-<平台>-<架构>` 形式的运行文件（Windows 为 `.exe`，macOS 为 `.zip`，Linux 无扩展名）。版本判断来自 GitHub Release Tag，而不是文件名；安装包、旧版带版本号资产、签名和其他附件都不会被选为自动更新 payload。下载内容继续使用同一 Release 中的 `SHA256SUMS` 校验。

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

Actions 自动安装 NSIS，并把两个文件重命名为固定 Release 资产名后一起发布和计算校验和。后续原地更新无需重新运行安装包，要求安装目录对当前用户可写。自行改为机器级安装或选择受保护目录可能导致更新权限不足。原地更新不会重新执行安装脚本，也不会自动刷新 Windows 卸载列表的版本号。

## Ubuntu DEB 与 macOS PKG

Actions 为 Ubuntu amd64 发布 `.deb`，为 macOS arm64 发布 `.pkg`，并保留对应免安装更新产物。macOS amd64 的 PKG/ZIP Task 仍可本地构建，但当前 Actions matrix 暂停自动发布。所有实际发布的安装包都包含在 `SHA256SUMS` 中。

Ubuntu 本地构建（默认 amd64，可传入实际目标架构）：

```bash
wails3 task linux:create:deb ARCH=amd64
sudo apt install ./bin/edgeinfer-node-test.deb
```

DEB 面向 Ubuntu 24.04+，依赖 `libgtk-4-1` 和 `libwebkitgtk-6.0-4`。程序安装到 `/usr/bin/<binaryName>`，应用菜单与图标安装到 `/usr/share`；卸载不删除用户目录中的配置。新版本通过再次安装新版 DEB 升级；仅下载 GitHub DEB 并不会自动配置 APT 软件源。

macOS 本地构建（必须在 macOS 上安装 Xcode Command Line Tools）：

```bash
wails3 task darwin:package:pkg ARCH=arm64
# Intel Mac 如需本地构建可使用 ARCH=amd64
```

生成 `bin/<binaryName>-<arch>.pkg`，双击安装到 `/Applications/<binaryName>.app`。构建使用 `pkgbuild`，禁用 bundle relocation，避免安装器误将 Downloads 下的旧副本作为目标。PKG 本身尚未使用 Developer ID Installer 证书签名或公证，内部 `.app` 沿用现有 ad-hoc 签名；正式公开分发需配置相应签名与公证，否则可能被 Gatekeeper 阻止。

**更新行为：** `/usr/bin` 下的本应用和 `/Applications` 下的应用禁用自动原地替换；界面显示“下载新版安装包”，点击主界面或托盘的更新入口会打开 Releases 下载页面。使用新版 DEB/PKG 升级，保持权限和系统安装记录一致。用户目录中的免安装二进制 / `.app` 仍使用原来的应用内更新。用户配置路径仍为 `~/.config/<configDirName>/settings.json`。
