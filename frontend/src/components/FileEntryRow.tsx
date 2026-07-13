import { FileEntry } from '../../bindings/androidfs/internal/model/models.js'

export function FileEntryRow({ entry, onOpen, selected, onSelect, onContextMenu }: {
  entry: FileEntry
  onOpen: () => void
  selected: boolean
  onSelect: () => void
  onContextMenu: (e: React.MouseEvent) => void
}) {
  return (
    <div
      className={['row', selected ? 'row-selected' : ''].join(' ')}
      onClick={onSelect}
      onDoubleClick={entry.is_dir ? onOpen : undefined}
      onContextMenu={onContextMenu}
    >
      <span className="row-icon" aria-hidden>{entry.is_dir ? '▸' : '·'}</span>
      <span className="row-name">
        {entry.name}
        {entry.link ? <span className="row-link mono"> → {entry.link}</span> : null}
      </span>
      <span className="row-size mono">{entry.is_dir ? '' : formatSize(entry.size)}</span>
      <span className="row-time mono">{formatTime(entry.mod_time)}</span>
    </div>
  )
}

function formatSize(n: number): string {
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB'
  return (n / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

// formatTime renders mod_time as "YYYY-MM-DD HH:mm". mod_time arrives as an ISO
// string (Go time.Time marshalled by wails). Missing/unparseable → empty, so
// the time column stays blank rather than printing "Invalid Date".
function formatTime(t: FileEntry['mod_time']): string {
  if (!t) return ''
  const ms = Date.parse(t as any)
  if (Number.isNaN(ms)) return ''
  const d = new Date(ms)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}
