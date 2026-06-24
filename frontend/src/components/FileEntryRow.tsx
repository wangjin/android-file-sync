import { FileEntry } from '../../bindings/androidfs/internal/model/models.js'

export function FileEntryRow({ entry, onOpen, selected, onSelect }: {
  entry: FileEntry
  onOpen: () => void
  selected: boolean
  onSelect: () => void
}) {
  return (
    <div
      className={['row', selected ? 'row-selected' : ''].join(' ')}
      onClick={onSelect}
      onDoubleClick={entry.is_dir ? onOpen : undefined}
    >
      <span className="row-icon" aria-hidden>{entry.is_dir ? '▸' : '·'}</span>
      <span className="row-name">
        {entry.name}
        {entry.link ? <span className="row-link mono"> → {entry.link}</span> : null}
      </span>
      <span className="row-size mono">{entry.is_dir ? '' : formatSize(entry.size)}</span>
    </div>
  )
}

function formatSize(n: number): string {
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  if (n < 1024 * 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB'
  return (n / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}
