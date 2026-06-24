import { useState, useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { useDevices } from './hooks/useDevices'
import { useDeviceBrowser } from './hooks/useDeviceBrowser'
import { useLocalBrowser } from './hooks/useLocalBrowser'
import { useTransfers } from './hooks/useTransfers'
import { PushFiles, PullFiles, Delete, DeleteLocal } from '../bindings/androidfs/app.js'
import { Toolbar } from './components/Toolbar'
import { FilePanel } from './components/FilePanel'
import { TransferTelemetry } from './components/TransferTelemetry'
import { ConnectDialog } from './components/ConnectDialog'
import { ConfirmDialog } from './components/ConfirmDialog'
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

  // Delete confirmation. `pendingDelete` holds which side+path is awaiting the
  // user's second click; null when the dialog is closed.
  const [pendingDelete, setPendingDelete] = useState<{ side: 'local' | 'device'; name: string; path: string } | null>(null)

  const requestDeleteLocal = () => {
    const e = local.entries.find(x => x.path === localSelected)
    if (e) setPendingDelete({ side: 'local', name: e.name, path: e.path })
  }
  const requestDeleteDevice = () => {
    const e = device.entries.find(x => x.path === deviceSelected)
    if (e) setPendingDelete({ side: 'device', name: e.name, path: e.path })
  }

  const confirmDelete = async () => {
    if (!pendingDelete) return
    const { side, path } = pendingDelete
    setPendingDelete(null)
    if (side === 'local') {
      await DeleteLocal(path)
      setLocalSelected(null)
      local.refresh()
    } else {
      await Delete(serial!, path)
      setDeviceSelected(null)
      device.refresh()
    }
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
          <button className="danger" onClick={requestDeleteLocal} disabled={!localSelected}>删除本地</button>
          <button className="danger" onClick={requestDeleteDevice} disabled={!deviceSelected}>删除设备</button>
        </footer>
      )}

      <TransferTelemetry tasks={tasks} />
      <ConnectDialog open={showConnect} onClose={() => setShowConnect(false)} />
      <ConfirmDialog
        open={pendingDelete !== null}
        title="确认删除"
        message={pendingDelete ? (
          <>确定删除 <strong>{pendingDelete.name}</strong>?<br />此操作不可撤销。</>
        ) : null}
        confirmLabel="删除"
        onConfirm={confirmDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </div>
  )
}
