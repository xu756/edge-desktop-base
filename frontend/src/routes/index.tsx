import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { DesktopService, type DesktopState } from '@/lib/desktop'
import { Events } from '@wailsio/runtime'
import { createFileRoute } from '@tanstack/react-router'
import {
  Cable,
  CheckCircle2,
  Download,
  LoaderCircle,
  MonitorCog,
  Power,
  RefreshCw,
  Server,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

export const Route = createFileRoute('/')({ component: Home })

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}

function Home() {
  const [state, setState] = useState<DesktopState | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState<string | null>(null)
  const [notice, setNotice] = useState('')
  const [ipcResult, setIpcResult] = useState('尚未测试')
  const [wsResult, setWsResult] = useState('尚未连接')
  const socketRef = useRef<WebSocket | null>(null)

  const refresh = async () => {
    try {
      setState(await DesktopService.State())
    } catch (error) {
      setNotice(errorMessage(error))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void refresh()
    const subscriptions = [
      Events.On('wails:updater:check-started', () => setNotice('正在检查更新…')),
      Events.On('wails:updater:no-update', () => setNotice('当前已是最新版本')),
      Events.On('wails:updater:update-available', () => setNotice('发现新版本，可打开更新窗口查看详情')),
      Events.On('wails:updater:download-started', () => setNotice('正在下载更新…')),
      Events.On('wails:updater:update-ready', () => setNotice('更新已下载并校验，重启应用后生效')),
      Events.On('wails:updater:error', (event) => {
        const data = event.data as { message?: string } | undefined
        setNotice(`更新失败：${data?.message ?? '请查看更新窗口'}`)
      }),
    ]
    return () => {
      socketRef.current?.close()
      subscriptions.forEach((unsubscribe) => unsubscribe())
    }
  }, [])

  const setAutoStart = async (enabled: boolean) => {
    setSaving('autostart')
    try {
      await DesktopService.SetAutoStart(enabled)
      await refresh()
      setNotice(enabled ? '已开启开机自启动' : '已关闭开机自启动')
    } catch (error) {
      setNotice(errorMessage(error))
    } finally {
      setSaving(null)
    }
  }

  const setCloseToTray = async (enabled: boolean) => {
    setSaving('tray')
    try {
      await DesktopService.SetCloseToTray(enabled)
      await refresh()
      setNotice(enabled ? '关闭窗口时将隐藏到系统托盘' : '关闭窗口时将直接退出')
    } catch (error) {
      setNotice(errorMessage(error))
    } finally {
      setSaving(null)
    }
  }

  const setAutoStartShowWindow = async (enabled: boolean) => {
    setSaving('autostart-window')
    try {
      await DesktopService.SetAutoStartShowWindow(enabled)
      await refresh()
      setNotice(enabled ? '自启动时将显示主窗口' : '自启动时将在托盘后台运行')
    } catch (error) {
      setNotice(errorMessage(error))
      await refresh()
    } finally {
      setSaving(null)
    }
  }

  const setAutoCheckUpdates = async (enabled: boolean) => {
    setSaving('auto-check-updates')
    try {
      await DesktopService.SetAutoCheckUpdates(enabled)
      await refresh()
      setNotice(enabled ? '已开启自动检查更新' : '已关闭自动检查更新')
    } catch (error) {
      setNotice(errorMessage(error))
    } finally {
      setSaving(null)
    }
  }

  const setAutoDownloadUpdates = async (enabled: boolean) => {
    setSaving('auto-download-updates')
    try {
      await DesktopService.SetAutoDownloadUpdates(enabled)
      await refresh()
      setNotice(enabled ? '发现新版本后将自动下载，但不会自动重启' : '发现新版本后只提醒，不自动下载')
    } catch (error) {
      setNotice(errorMessage(error))
    } finally {
      setSaving(null)
    }
  }

  const checkUpdate = async () => {
    setSaving('update')
    try {
      setNotice('正在打开更新窗口…')
      await DesktopService.CheckForUpdates()
    } catch (error) {
      setNotice(errorMessage(error))
    } finally {
      setSaving(null)
    }
  }

  const testIPC = async () => {
    try {
      setIpcResult(await DesktopService.Echo('hello from frontend'))
    } catch (error) {
      setIpcResult(errorMessage(error))
    }
  }

  const testWebSocket = () => {
    if (!state?.websocketURL) return
    socketRef.current?.close()
    setWsResult('连接中…')

    const socket = new WebSocket(state.websocketURL)
    socketRef.current = socket
    socket.onopen = () => {
      setWsResult('已连接，等待消息…')
      socket.send(JSON.stringify({ type: 'ping', payload: 'hello from frontend' }))
    }
    socket.onmessage = (event) => setWsResult(event.data)
    socket.onerror = () => setWsResult('WebSocket 连接失败')
    socket.onclose = () => {
      if (socketRef.current === socket) socketRef.current = null
    }
  }

  return (
    <main className='mx-auto flex min-h-screen w-full max-w-5xl flex-col gap-6 px-6 py-8'>
      <header className='flex items-start justify-between gap-4'>
        <div className='space-y-1'>
          <div className='flex items-center gap-2'>
            <MonitorCog className='size-5' />
            <h1 className='text-xl font-semibold tracking-tight'>{state?.name ?? '加载中…'}</h1>
          </div>
          <p className='text-muted-foreground text-sm'>Wails v3 桌面应用基础底座</p>
        </div>
        <Badge>{state ? `v${state.version}` : '加载中'}</Badge>
      </header>

      {notice ? (
        <div className='bg-muted/60 text-muted-foreground rounded-lg border px-4 py-3 text-sm'>
          {notice}
        </div>
      ) : null}

      <section className='grid gap-5 md:grid-cols-2'>
        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2'>
              <Power className='size-4' />
              桌面设置
            </CardTitle>
            <CardDescription>底座默认提供的系统级能力。</CardDescription>
          </CardHeader>
          <CardContent>
            <SettingRow
              title='开机自动启动'
              description='登录系统时自动运行本程序。'
              checked={state?.autoStart ?? false}
              disabled={loading || saving !== null}
              onCheckedChange={setAutoStart}
            />
            <SettingRow
              title='自启动时显示主窗口'
              description={state?.autoStartWindowSupported === false
                ? '当前 macOS 登录项方式不支持窗口参数，由系统控制窗口显示。'
                : '关闭后自启动仅显示托盘图标；手动打开程序仍显示窗口。'}
              checked={state?.autoStartShowWindow ?? false}
              disabled={loading || saving !== null || !!state?.configError || state?.autoStartWindowSupported === false}
              onCheckedChange={setAutoStartShowWindow}
            />
            {state?.autoStartError ? <p role='alert' className='text-destructive text-sm'>{state.autoStartError}</p> : null}
            <SettingRow
              title='关闭到系统托盘'
              description='点击窗口关闭按钮时隐藏窗口，托盘菜单仍可重新打开。'
              checked={state?.closeToTray ?? true}
              disabled={loading || saving !== null || !!state?.configError}
              onCheckedChange={setCloseToTray}
            />
            <div className='flex items-center justify-between rounded-lg border p-3'>
              <div>
                <div className='text-sm font-medium'>系统托盘</div>
                <div className='text-muted-foreground mt-1 text-xs'>打开主窗口 / 检查更新 / 退出</div>
              </div>
              <Badge>{state?.trayReady ? '已启用' : '初始化中'}</Badge>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2'>
              <Download className='size-4' />
              软件更新
            </CardTitle>
            <CardDescription>{state?.updateInstallHint || 'CNB 公共分发仓库 + SHA-256 校验；下载完成后由你决定何时重启。'}</CardDescription>
          </CardHeader>
          <CardContent>
            <SettingRow
              title='自动检查更新'
              description={`每 ${state?.updateIntervalHours ?? 6} 小时在后台检查新版本，不主动弹窗。`}
              checked={state?.autoCheckUpdates ?? true}
              disabled={loading || saving !== null || !!state?.configError}
              onCheckedChange={setAutoCheckUpdates}
            />
            <SettingRow
              title='自动下载更新'
              description='发现新版本后在后台下载并校验；下载完成后不会自动重启。'
              checked={state?.autoDownloadUpdates ?? false}
              disabled={loading || saving !== null || !!state?.configError || state?.autoCheckUpdates === false}
              onCheckedChange={setAutoDownloadUpdates}
            />
            <InfoRow label='当前版本' value={state ? `v${state.version}` : '-'} />
            <InfoRow label='检查间隔' value={state ? `${state.updateIntervalHours} 小时` : '-'} />
            <InfoRow label='更新源' value={state?.updateRepositoryURL ?? '-'} />
            <Button onClick={checkUpdate} disabled={loading || saving !== null} className='w-fit'>
              {saving === 'update' ? (
                <LoaderCircle className='animate-spin' data-icon='inline-start' />
              ) : (
                <RefreshCw data-icon='inline-start' />
              )}
              检查更新
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2'>
              <Server className='size-4' />
              前端调用 Go
            </CardTitle>
            <CardDescription>业务页面直接使用 Wails 生成的类型安全 bindings。</CardDescription>
          </CardHeader>
          <CardContent>
            <div className='bg-muted/60 min-h-12 rounded-lg border p-3 font-mono text-xs'>{ipcResult}</div>
            <Button variant='outline' onClick={testIPC} className='w-fit'>
              <CheckCircle2 data-icon='inline-start' />
              测试服务调用
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2'>
              <Cable className='size-4' />
              WebSocket
            </CardTitle>
            <CardDescription>Gin 本地服务仅监听 loopback，后续可扩展给算法进程或本机服务。</CardDescription>
          </CardHeader>
          <CardContent>
            <InfoRow label='地址' value={state?.websocketURL ?? '-'} />
            <InfoRow label='服务状态' value={state?.localServer.running ? '运行中' : '未启动'} />
            <div className='bg-muted/60 max-h-24 min-h-12 overflow-auto rounded-lg border p-3 font-mono text-xs'>
              {wsResult}
            </div>
            <Button variant='outline' onClick={testWebSocket} disabled={!state?.localServer.running} className='w-fit'>
              <Cable data-icon='inline-start' />
              连接并发送 Ping
            </Button>
          </CardContent>
        </Card>
      </section>

      <Card>
        <CardHeader>
          <CardTitle>配置文件</CardTitle>
          <CardDescription>保存在当前用户目录，自更新后继续保留。手动编辑后重启程序生效。</CardDescription>
        </CardHeader>
        <CardContent>
          <code className='select-text break-all rounded-lg border p-3 text-xs'>{state?.configPath || '配置目录不可用'}</code>
          {state?.configError ? <p role='alert' className='text-destructive text-sm'>配置读取失败，暂用默认设置且禁止覆盖原文件。请修复配置后重启：{state.configError}</p> : null}
        </CardContent>
      </Card>

      <footer className='text-muted-foreground flex flex-wrap gap-x-5 gap-y-1 border-t pt-4 text-xs'>
        <span>{state ? `${state.platform}/${state.architecture}` : '-'}</span>
        <span>Commit {state?.commit ? state.commit.slice(0, 12) : '-'}</span>
        <span>Build {state?.buildTime ?? '-'}</span>
      </footer>
    </main>
  )
}

function SettingRow({
  title,
  description,
  checked,
  disabled,
  onCheckedChange,
}: {
  title: string
  description: string
  checked: boolean
  disabled?: boolean
  onCheckedChange: (checked: boolean) => void
}) {
  return (
    <label className='flex cursor-pointer items-center justify-between gap-4 rounded-lg border p-3'>
      <span>
        <span className='block text-sm font-medium'>{title}</span>
        <span className='text-muted-foreground mt-1 block text-xs leading-5'>{description}</span>
      </span>
      <Switch checked={checked} disabled={disabled} onCheckedChange={onCheckedChange} />
    </label>
  )
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className='flex items-center justify-between gap-4 text-sm'>
      <span className='text-muted-foreground'>{label}</span>
      <span className='max-w-[65%] truncate font-medium'>{value}</span>
    </div>
  )
}
