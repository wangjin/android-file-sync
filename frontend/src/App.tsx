import { useState, useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { useDevices } from './hooks/useDevices'
import { useDeviceBrowser } from './hooks/useDeviceBrowser'
import { useLocalBrowser } from './hooks/useLocalBrowser'
import { useTransfers } from './hooks/useTransfers'
import { PushFiles, PullFiles, Delete, DeleteLocal } from '../bindings/androidfs/app.js'
import { FileEntry } from '../bindings/androidfs/internal/model/models.js'
import { Toolbar } from './components/Toolbar'
import { FilePanel } from './components/FilePanel'
import { TransferTelemetry } from './components/TransferTelemetry'
import { ConnectDialog } from './components/ConnectDialog'
import { ConfirmDialog } from './components/ConfirmDialog'
import { ContextMenu, MenuItem } from './components/ContextMenu'
import { EmptyState } from './components/EmptyState'

// A pending context menu: which side it opened on, and the cursor position.
interface MenuState { side: 'local' | 'device'; entry: FileEntry; x: number; y: number }

export default function App() {
  const { devices } = useDevices()
  const { tasks } = useTransfers()
  const local = useLocalBrowser()
  const [serial, setSerial] = useState<string | null>(null)
  const [showConnect, setShowConnect] = useState(false)
  const device = useDeviceBrowser(serial)

  const [localSelected, setLocalSelected] = useState<string | null>(null)
  const [deviceSelected, setDeviceSelected] = useState<string | null>(null)

  // Right-click context menu + delete confirmation.
  const [menu, setMenu] = useState<MenuState | null>(null)
  const [pendingDelete, setPendingDelete] = useState<{ side: 'local' | 'device'; name: string; path: string } | null>(null)

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

  useEffect(() => {
    if (serial) device.refresh()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [serial])

  // ===== Transfers =====
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

  // ===== Delete (with two-step confirm) =====
  const requestDelete = (side: 'local' | 'device', entry: FileEntry) => {
    setPendingDelete({ side, name: entry.name, path: entry.path })
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

  // ===== Context menu =====
  // Local entry: push to device, delete. Device entry: pull to local, delete.
  const openMenu = (side: 'local' | 'device', entry: FileEntry, e: React.MouseEvent) => {
    e.preventDefault()
    if (side === 'local') setLocalSelected(entry.path)
    else setDeviceSelected(entry.path)
    setMenu({ side, entry, x: e.clientX, y: e.clientY })
  }

  function buildMenuItems(side: 'local' | 'device', entry: FileEntry): MenuItem[] {
    if (side === 'local') {
      return [
        { label: '↑ 推送至设备', onSelect: pushSelected, disabled: !serial },
        { label: '删除', onSelect: () => requestDelete('local', entry), danger: true },
      ]
    }
    return [
      { label: '↓ 拉取到本地', onSelect: pullSelected, disabled: !serial },
      { label: '删除', onSelect: () => requestDelete('device', entry), danger: true },
    ]
  }

  const menuItems: MenuItem[] = menu ? buildMenuItems(menu.side, menu.entry) : []

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
        <EmptyState message="用 USB 连接设备并开启 USB 调试,或点右上「无线连接」。右键文件可推送/拉取/删除。" />
      ) : (
        <main className="panes">
          <FilePanel title="本地" path={local.root || '~'} entries={local.entries} loading={local.loading} error={local.error}
            onNavigate={local.navigate} onOpen={local.enter} selectedPath={localSelected} onSelect={setLocalSelected}
            onRowContextMenu={(entry, e) => openMenu('local', entry, e)} />
          <div className="seam" aria-hidden />
          <FilePanel title="设备" path={device.path} entries={device.entries} loading={device.loading} error={device.error}
            onNavigate={device.navigate} onOpen={device.enter} selectedPath={deviceSelected} onSelect={setDeviceSelected}
            onRowContextMenu={(entry, e) => openMenu('device', entry, e)} />
        </main>
      )}

      <TransferTelemetry tasks={tasks} />

      {menu && (
        <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />
      )}
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
