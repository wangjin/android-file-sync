import { FileEntry } from '../../bindings/androidfs/internal/model/models.js'
import { FileEntryRow } from './FileEntryRow'
import { PathBreadcrumb } from './PathBreadcrumb'
import { useSort, sortEntries, SortKey } from '../hooks/useSort'

export function FilePanel({ title, path, entries, loading, error, onNavigate, onOpen, selectedPath, onSelect, onRowContextMenu }: {
  title: string
  path: string
  entries: FileEntry[]
  loading: boolean
  error: string | null
  onNavigate: (p: string) => void
  onOpen: (name: string) => void
  selectedPath: string | null
  onSelect: (p: string) => void
  onRowContextMenu: (entry: FileEntry, e: React.MouseEvent) => void
}) {
  const { sort, toggle } = useSort()
  const sorted = sortEntries(entries, sort)

  const arrow = (key: SortKey) => sort.key === key ? (sort.dir === 'asc' ? ' ▲' : ' ▼') : ''

  return (
    <section className="panel">
      <header className="panel-head">
        <span className="panel-title">{title}</span>
        <PathBreadcrumb path={path} onNavigate={onNavigate} />
        {loading && <span className="badge mono">loading</span>}
      </header>
      <div className="sort-bar">
        <button className="sort-cell sort-name" onClick={() => toggle('name')}>名称{arrow('name')}</button>
        <button className="sort-cell sort-size" onClick={() => toggle('size')}>大小{arrow('size')}</button>
        <button className="sort-cell sort-time" onClick={() => toggle('time')}>修改时间{arrow('time')}</button>
      </div>
      <div className="panel-body">
        {error && <div className="panel-error">{error}</div>}
        {!loading && sorted.length === 0 && !error && (
          <div className="panel-empty muted">空</div>
        )}
        {sorted.map(e => (
          <FileEntryRow key={e.path} entry={e} selected={selectedPath === e.path}
            onSelect={() => onSelect(e.path)} onOpen={() => onOpen(e.name)}
            onContextMenu={(ev) => onRowContextMenu(e, ev)} />
        ))}
      </div>
    </section>
  )
}
