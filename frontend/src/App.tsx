import { useState, useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { useDevices } from './hooks/useDevices'
import { useDeviceBrowser } from './hooks/useDeviceBrowser'
import { useLocalBrowser } from './hooks/useLocalBrowser'
import { useTransfers } from './hooks/useTransfers'
import { PushFiles, PullFiles } from '../bindings/androidfs/app.js'
import { Toolbar } from './components/Toolbar'
import { FilePanel } from './components/FilePanel'
import { TransferTelemetry } from './components/TransferTelemetry'
import { ConnectDialog } from './components/ConnectDialog'
import { EmptyState } from './components/EmptyState'

export default function App() {
  const { devices } = useDevices()
  const { tasks } = useTransfers()
  const local = useLocalBrowser()
  const [serial, setSerial] = useState<string | null>(null)
  const [showConnect, setShowConnect] = useState(false)
  const device = useDeviceBrowser(serial)

  const [localSelected, setLocalSelected] = useState<string | null>(null)
  const [deviceSelected, setDeviceSelected] = useState<string | null>(null)

  useEffect(() => {
    const cancel = Events.On('files-dropped', async (ev: any) => {
      const files: string[] = ev.data?.files ?? []
      if (serial && device.path) {
        await PushFiles(serial, files, device.path)
        device.refresh()
      }
    })
    return () => { cancel() }
  }, [serial, device.path])

  // Auto-list device dir when a device is chosen or path changes.
  useEffect(() => {
    if (serial) device.refresh()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [serial])

  const pushSelected = async () => {
    if (!serial || !localSelected) return
    await PushFiles(serial, [localSelected], device.path)
    device.refresh()
  }
  const pullSelected = async () => {
    if (!serial || !deviceSelected) return
    await PullFiles(serial, [deviceSelected], local.root || '~/Downloads')
    local.refresh()
  }

  return (
    <div className="app-root">
      <Toolbar
        devices={devices}
        selected={serial}
        onSelect={setSerial}
        onRefresh={() => (serial ? device.refresh() : location.reload())}
        onConnect={() => setShowConnect(true)}
      />

      {!serial ? (
        <EmptyState message="用 USB 连接设备并开启 USB 调试,或点右上「无线连接」。" />
      ) : (
        <main className="panes">
          <FilePanel title="本地" path={local.root || '~'} entries={local.entries} loading={local.loading} error={local.error}
            onNavigate={local.navigate} onOpen={local.enter} selectedPath={localSelected} onSelect={setLocalSelected} />
          <div className="seam" aria-hidden />
          <FilePanel title="设备" path={device.path} entries={device.entries} loading={device.loading} error={device.error}
            onNavigate={device.navigate} onOpen={device.enter} selectedPath={deviceSelected} onSelect={setDeviceSelected} />
        </main>
      )}

      {serial && (
        <footer className="actions">
          <button onClick={pushSelected} disabled={!localSelected}>↑ 推送至设备</button>
          <button onClick={pullSelected} disabled={!deviceSelected}>↓ 拉取到本地</button>
        </footer>
      )}

      <TransferTelemetry tasks={tasks} />
      <ConnectDialog open={showConnect} onClose={() => setShowConnect(false)} />
    </div>
  )
}
