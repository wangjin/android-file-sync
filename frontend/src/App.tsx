import { useState, useEffect, useRef } from 'react'
import { Events } from '@wailsio/runtime'
import { useDevices } from './hooks/useDevices'
import { useDeviceBrowser } from './hooks/useDeviceBrowser'
import { useLocalBrowser } from './hooks/useLocalBrowser'
import { useTransfers } from './hooks/useTransfers'
import { PushFiles, PullFiles, Delete, DeleteLocal, Rename, RenameLocal } from '../bindings/androidfs/app.js'
import { FileEntry } from '../bindings/androidfs/internal/model/models.js'
import { Toolbar } from './components/Toolbar'
import { ModuleBar, ModuleId } from './components/ModuleBar'
import { FilePanel } from './components/FilePanel'
import { TransferTelemetry } from './components/TransferTelemetry'
import { ConnectDialog } from './components/ConnectDialog'
import { ConfirmDialog } from './components/ConfirmDialog'
import { PromptDialog } from './components/PromptDialog'
import { ContextMenu, MenuItem } from './components/ContextMenu'
import { EmptyState } from './components/EmptyState'
import { Toast } from './components/Toast'
import { UpdateDialog } from './components/UpdateDialog'
import { ImagePreview } from './components/ImagePreview'
import { MonitorCharts } from './components/MonitorPanel'
import { isImageFile } from './lib/image'
import { useUpdate } from './hooks/useUpdate'
import { useMonitor, MonitorSample } from './hooks/useMonitor'
import { Version } from '../bindings/androidfs/app.js'

// A pending context menu: which side it opened on, and the cursor position.
interface MenuState { side: 'local' | 'device'; entry: FileEntry; x: number; y: number }

export default function App() {
  const { devices } = useDevices()
  const { tasks, dismiss } = useTransfers()
  const local = useLocalBrowser()
  const [serial, setSerial] = useState<string | null>(null)
  const [showConnect, setShowConnect] = useState(false)
  const device = useDeviceBrowser(serial)

  const update = useUpdate()
  const monitor = useMonitor()
  const [appVersion, setAppVersion] = useState('dev')
  useEffect(() => { Version().then(setAppVersion) }, [])

  // Top-level module: the file manager (browse + transfer) and the resource
  // monitor are peers — switching modules swaps the whole work area, so
  // neither one competes with the other for layout space.
  const [module, setModule] = useState<ModuleId>('files')

  // Latest monitor sample, surfaced as a mini readout on the monitor tab chip
  // so the data stays visible even while the files module is active.
  const monitorLatest = monitor.samples.length
    ? monitor.samples[monitor.samples.length - 1]
    : null

  // Each pane tracks its own selection as a path set — empty = none selected.
  const [localSelected, setLocalSelected] = useState<Set<string>>(new Set())
  const [deviceSelected, setDeviceSelected] = useState<Set<string>>(new Set())

  // Right-click context menu + delete confirmation + rename prompt.
  const [menu, setMenu] = useState<MenuState | null>(null)
  const [pendingDelete, setPendingDelete] = useState<{ side: 'local' | 'device'; paths: string[] } | null>(null)
  const [pendingRename, setPendingRename] = useState<{ side: 'local' | 'device'; entry: FileEntry } | null>(null)
  // In-app image preview (device OR local). Images are fetched as raw bytes,
  // not downloaded to disk — the blob is in-memory only.
  const [preview, setPreview] = useState<{ side: 'local' | 'device'; entry: FileEntry } | null>(null)

  useEffect(() => {
    const cancel = Events.On('files-dropped', async (ev: any) => {
      const files: string[] = ev.data?.files ?? []
      // Push is async; the task:done listener re-lists the device pane on finish.
      if (serial && device.path) await PushFiles(serial, files, device.path)
    })
    return () => { cancel() }
  }, [serial, device.path])

  useEffect(() => {
    if (serial) device.refresh()
    // Selection belongs to the previous listing — clear it when the device
    // changes so stale paths can't drive a delete or transfer.
    setDeviceSelected(new Set())
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [serial])

  // Navigating into another directory invalidates every selected path, so both
  // panes wrap their navigate fn to clear selection first.
  const navigateLocal = (p: string) => { setLocalSelected(new Set()); local.navigate(p) }
  const navigateDevice = (p: string) => { setDeviceSelected(new Set()); device.navigate(p) }

  // Refresh the affected pane when a transfer actually finishes. Transfers are
  // async on the backend, so awaiting PushFiles/PullFiles returns before the
  // copy is done — we listen for the backend's task:done event instead.
  // Refs hold the latest refresh fns so the subscription runs once.
  const paneRef = useRef({ local: () => local.refresh(), device: () => device.refresh() })
  paneRef.current = { local: () => local.refresh(), device: () => device.refresh() }
  useEffect(() => {
    const cancel = Events.On('task:done', (ev: any) => {
      const side: 'local' | 'device' = ev.data?.side
      if (side === 'local') paneRef.current.local()
      else if (side === 'device') paneRef.current.device()
    })
    return () => { cancel() }
  }, [])

  // ===== Transfers =====
  // PushFiles/PullFiles already take path arrays on the backend, so a batch is
  // just "spread the selection"; each path becomes its own transfer task.
  const pushSelected = async () => {
    const paths = [...localSelected]
    if (!serial || paths.length === 0) return
    // Refresh happens on task:done, not here — push is async.
    await PushFiles(serial, paths, device.path)
  }
  const pullSelected = async () => {
    const paths = [...deviceSelected]
    if (!serial || paths.length === 0) return
    await PullFiles(serial, paths, local.root || '~/Downloads')
  }

  // ===== Delete (with two-step confirm) =====
  // requestDelete runs against the pane's whole selection. The right-clicked
  // entry is guaranteed to be in the set by openMenu (Finder behavior).
  const requestDelete = (side: 'local' | 'device') => {
    const paths = [...(side === 'local' ? localSelected : deviceSelected)]
    if (paths.length === 0) return
    setPendingDelete({ side, paths })
  }
  const confirmDelete = async () => {
    if (!pendingDelete) return
    const { side, paths } = pendingDelete
    setPendingDelete(null)
    // Delete one path at a time — backend Delete/DeleteLocal take a single
    // path. A failure is reported but does not abort the remaining items.
    let firstErr = ''
    for (const p of paths) {
      try {
        if (side === 'local') await DeleteLocal(p)
        else await Delete(serial!, p)
      } catch (e: any) {
        firstErr ||= e?.message ?? String(e)
      }
    }
    if (side === 'local') {
      setLocalSelected(new Set())
      local.refresh()
    } else {
      setDeviceSelected(new Set())
      device.refresh()
    }
    if (firstErr) console.error('删除失败:', firstErr)
  }

  // ===== Rename (prompt for new name; keep it in the same directory) =====
  const requestRename = (side: 'local' | 'device', entry: FileEntry) => {
    setPendingRename({ side, entry })
  }
  const confirmRename = async (newName: string) => {
    const p = pendingRename
    if (!p) return
    setPendingRename(null)
    const { entry } = p
    if (newName === entry.name) return
    // Replace the basename, keep the parent dir. lastIndexOf includes the "/"
    // so a root entry like "/foo.txt" keeps parent "/", and "/sdcard/x" → "/sdcard".
    const slash = entry.path.lastIndexOf('/')
    const newPath = slash >= 0 ? entry.path.slice(0, slash + 1) + newName : newName
    if (p.side === 'local') {
      await RenameLocal(entry.path, newPath)
      local.refresh()
    } else {
      await Rename(serial!, entry.path, newPath)
      device.refresh()
    }
  }

  // ===== Copy file name (single-only, like rename) =====
  // Writes just the basename to the system clipboard. clipboard.writeText can
  // reject in non-secure webview contexts; swallow the error so the menu stays
  // non-blocking (no toast — failure is rare and low-impact).
  const copyName = (entry: FileEntry) => {
    navigator.clipboard?.writeText(entry.name).catch(() => {})
  }

  // ===== Context menu =====
  // Local entry: push to device, delete. Device entry: pull to local, delete.
  // Right-clicking a row that isn't selected selects just that row (Finder);
  // right-clicking inside an existing multi-selection preserves the selection.
  const openMenu = (side: 'local' | 'device', entry: FileEntry, e: React.MouseEvent) => {
    e.preventDefault()
    const cur = side === 'local' ? localSelected : deviceSelected
    if (!cur.has(entry.path)) {
      const only = new Set([entry.path])
      if (side === 'local') setLocalSelected(only)
      else setDeviceSelected(only)
    }
    setMenu({ side, entry, x: e.clientX, y: e.clientY })
  }

  function buildMenuItems(side: 'local' | 'device', entry: FileEntry): MenuItem[] {
    const count = (side === 'local' ? localSelected : deviceSelected).size || 1
    // Rename / preview are single-entry semantics — only offered for one row.
    const single = count === 1
    // Images get a "preview" entry at the top (opens the in-app lightbox; no
    // download). Other file types keep the original transfer/rename/delete set.
    const head: MenuItem[] = single && isImageFile(entry)
      ? [{ label: '👁 预览', onSelect: () => setPreview({ side, entry }) }]
      : []
    // For batches, append the count so the menu reflects how many items the
    // action will touch.
    const xfer = count > 1 ? ` (${count} 项)` : ''
    if (side === 'local') {
      return [
        ...head,
        { label: `↑ 推送至设备${xfer}`, onSelect: pushSelected, disabled: !serial },
        ...(single ? [
          { label: '📋 复制文件名', onSelect: () => copyName(entry) },
          { label: '重命名', onSelect: () => requestRename('local', entry) },
        ] : []),
        { label: count > 1 ? `删除 (${count} 项)` : '删除', onSelect: () => requestDelete('local'), danger: true },
      ]
    }
    return [
      ...head,
      { label: `↓ 拉取到本地${xfer}`, onSelect: pullSelected, disabled: !serial },
      ...(single ? [
        { label: '📋 复制文件名', onSelect: () => copyName(entry) },
        { label: '重命名', onSelect: () => requestRename('device', entry) },
      ] : []),
      { label: count > 1 ? `删除 (${count} 项)` : '删除', onSelect: () => requestDelete('device'), danger: true },
    ]
  }

  const menuItems: MenuItem[] = menu ? buildMenuItems(menu.side, menu.entry) : []

  const startMonitor = () => {
    if (serial) monitor.start(serial)
  }

  return (
    <div className="app-root">
      <Toolbar
        devices={devices}
        selected={serial}
        onSelect={setSerial}
        onRefresh={() => {
          // Refresh both panes. Previously this only refreshed the device pane
          // (or did a full page reload when no device was connected), which left
          // the local pane stale. Reload the listing instead of the whole page
          // so selection/sort state survives.
          if (serial) device.refresh()
          local.refresh()
        }}
        onConnect={() => setShowConnect(true)}
        version={appVersion}
        onCheckUpdate={update.checkNow}
      />

      <ModuleBar
        module={module}
        onChange={setModule}
        monitorActive={monitor.running}
        monitorLatest={monitorLatest}
      />

      <main className="work-area">
        {module === 'files' ? (
          !serial ? (
            <EmptyState message="用 USB 连接设备并开启 USB 调试,或点右上「无线连接」。右键文件可推送/拉取/删除,双击图片可直接预览。" />
          ) : (
            <div className="files-area">
              <div className="panes">
                <FilePanel title="本地" path={local.root || '~'} entries={local.entries} loading={local.loading} error={local.error}
                  onNavigate={navigateLocal} onOpen={local.enter} selectedPaths={localSelected} onSelectChange={setLocalSelected}
                  onPreview={(entry) => setPreview({ side: 'local', entry })}
                  onRowContextMenu={(entry, e) => openMenu('local', entry, e)}
                  history={local.history} onClearHistory={local.clearHistory} />
                <div className="seam" aria-hidden />
                <FilePanel title="设备" path={device.path} entries={device.entries} loading={device.loading} error={device.error}
                  onNavigate={navigateDevice} onOpen={device.enter} selectedPaths={deviceSelected} onSelectChange={setDeviceSelected}
                  onPreview={(entry) => setPreview({ side: 'device', entry })}
                  onRowContextMenu={(entry, e) => openMenu('device', entry, e)}
                  history={device.history} onClearHistory={device.clearHistory} />
              </div>
              <TransferTelemetry tasks={tasks} onDismiss={dismiss} />
            </div>
          )
        ) : (
          <MonitorModule
            running={monitor.running}
            samples={monitor.samples}
            intervalMs={monitor.intervalMs}
            onIntervalChange={monitor.changeInterval}
            onStart={startMonitor}
            onStop={monitor.stop}
            canStart={!!serial}
            taskCount={tasks.length}
            onShowFiles={() => setModule('files')}
          />
        )}
      </main>

      {menu && (
        <ContextMenu x={menu.x} y={menu.y} items={menuItems} onClose={() => setMenu(null)} />
      )}
      <ConnectDialog open={showConnect} onClose={() => setShowConnect(false)} />
      <ConfirmDialog
        open={pendingDelete !== null}
        title="确认删除"
        message={pendingDelete ? (
          pendingDelete.paths.length > 1 ? (
            <>确定删除选中的 <strong>{pendingDelete.paths.length}</strong> 项？<br />此操作不可撤销。</>
          ) : (
            <>确定删除 <strong>{pendingDelete.paths[0].split('/').filter(Boolean).pop() ?? pendingDelete.paths[0]}</strong>?<br />此操作不可撤销。</>
          )
        ) : null}
        confirmLabel="删除"
        onConfirm={confirmDelete}
        onCancel={() => setPendingDelete(null)}
      />
      <PromptDialog
        open={pendingRename !== null}
        title="重命名"
        initialValue={pendingRename?.entry.name ?? ''}
        confirmLabel="重命名"
        onConfirm={confirmRename}
        onCancel={() => setPendingRename(null)}
      />
      {update.info && (
        <UpdateDialog info={update.info} onClose={update.dismissInfo} />
      )}
      {preview && (
        <ImagePreview
          entry={preview.entry}
          side={preview.side}
          serial={serial}
          onClose={() => setPreview(null)}
        />
      )}
      <Toast message={update.toast} />
    </div>
  )
}

// ===== Monitor module =====
// The monitor as a first-class module: a full-width instrument board instead
// of a squeezed bottom strip. The header carries the controls (start/stop +
// sampling interval); the body gives both sparklines the room they need.
function MonitorModule({ running, samples, intervalMs, onIntervalChange, onStart, onStop, canStart, taskCount, onShowFiles }: {
  running: boolean
  samples: MonitorSample[]
  intervalMs: number
  onIntervalChange: (ms: number) => void
  onStart: () => void
  onStop: () => void
  canStart: boolean
  taskCount: number
  onShowFiles: () => void
}) {
  const latest = samples.length ? samples[samples.length - 1] : null
  return (
    <section className="monitor-module">
      <header className="monitor-module-head">
        <span className="panel-title monitor-title">
          资源监控
          {running && <span className="monitor-live-dot" />}
        </span>
        {running && latest && (
          <span className="monitor-module-readout mono">
            CPU <strong className="monitor-readout-cpu">{latest.cpu != null ? `${latest.cpu.toFixed(1)}%` : '—'}</strong>
            <span className="module-tab-live-sep" aria-hidden>·</span>
            NPU <strong className="monitor-readout-npu">{latest.npu != null ? `${latest.npu.toFixed(1)}%` : '—'}</strong>
          </span>
        )}
        <span className="toolbar-spacer" />
        {taskCount > 0 && (
          <button className="btn-quiet" onClick={onShowFiles} title="切换回文件模块查看传输进度">
            {taskCount} 个传输任务 ↗
          </button>
        )}
        <select
          className="monitor-interval"
          value={intervalMs}
          onChange={e => onIntervalChange(Number(e.target.value))}
          disabled={!running}
        >
          <option value={500}>0.5s</option>
          <option value={1000}>1s</option>
          <option value={2000}>2s</option>
          <option value={5000}>5s</option>
        </select>
        {running
          ? <button className="telem-cancel" onClick={onStop}>■ 停止</button>
          : <button className="primary" onClick={onStart} disabled={!canStart}>开始监控</button>}
      </header>
      <div className="monitor-module-body">
        {running ? (
          <MonitorCharts samples={samples} />
        ) : (
          <div className="monitor-idle">
            <p>{canStart ? '采样尚未开始。' : '连接设备后即可采样。'}</p>
            {canStart && (
              <p className="muted">开始后将每 {intervalMs / 1000}s 采集一次设备的 CPU 与 NPU 占用。</p>
            )}
          </div>
        )}
      </div>
    </section>
  )
}
