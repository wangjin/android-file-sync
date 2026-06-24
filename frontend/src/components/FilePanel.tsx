import { FileEntry } from '../../bindings/androidfs/internal/model/models.js'
import { FileEntryRow } from './FileEntryRow'
import { PathBreadcrumb } from './PathBreadcrumb'

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
  return (
    <section className="panel">
      <header className="panel-head">
        <span className="panel-title">{title}</span>
        <PathBreadcrumb path={path} onNavigate={onNavigate} />
        {loading && <span className="badge mono">loading</span>}
      </header>
      <div className="panel-body">
        {error && <div className="panel-error">{error}</div>}
        {!loading && entries.length === 0 && !error && (
          <div className="panel-empty muted">空</div>
        )}
        {entries.map(e => (
          <FileEntryRow key={e.path} entry={e} selected={selectedPath === e.path}
            onSelect={() => onSelect(e.path)} onOpen={() => onOpen(e.name)}
            onContextMenu={(ev) => onRowContextMenu(e, ev)} />
        ))}
      </div>
    </section>
  )
}
