import { useState, useEffect } from 'react'
import { FileEntry } from '../../bindings/androidfs/internal/model/models.js'
import { FileEntryRow, SelectMods } from './FileEntryRow'
import { PathBreadcrumb } from './PathBreadcrumb'
import { useSort, sortEntries, SortKey } from '../hooks/useSort'

// Selection model: the parent owns the chosen set; this panel owns the range-
// selection anchor. Modifier handling follows the Finder convention:
//   - bare click  → select just this row (anchor = this row)
//   - Ctrl/Cmd    → toggle this row in the set (anchor = this row)
//   - Shift       → union the rows between anchor and this row with the set
export function FilePanel({ title, path, entries, loading, error, onNavigate, onOpen, onPreview, selectedPaths, onSelectChange, onRowContextMenu, history }: {
  title: string
  path: string
  entries: FileEntry[]
  loading: boolean
  error: string | null
  onNavigate: (p: string) => void
  onOpen: (name: string) => void
  // onPreview is offered for double-clickable files (images). Optional — when
  // omitted, double-click on a non-dir is a no-op (the original behavior).
  onPreview?: (entry: FileEntry) => void
  selectedPaths: Set<string>
  onSelectChange: (paths: Set<string>) => void
  onRowContextMenu: (entry: FileEntry, e: React.MouseEvent) => void
  history?: string[]
}) {
  const { sort, toggle } = useSort()
  const sorted = sortEntries(entries, sort)

  // Range selection needs a pivot; reset it whenever the listing changes so a
  // stale anchor can't point at a row that no longer exists.
  const [anchor, setAnchor] = useState<string | null>(null)
  useEffect(() => { setAnchor(null) }, [entries])

  const arrow = (key: SortKey) => sort.key === key ? (sort.dir === 'asc' ? ' ▲' : ' ▼') : ''

  const handleSelect = (entry: FileEntry, mods: SelectMods) => {
    if (mods.shift && anchor) {
      const paths = sorted.map(e => e.path)
      const a = paths.indexOf(anchor)
      const b = paths.indexOf(entry.path)
      if (a >= 0 && b >= 0) {
        const [lo, hi] = a < b ? [a, b] : [b, a]
        const next = new Set(selectedPaths)
        for (let i = lo; i <= hi; i++) next.add(paths[i])
        onSelectChange(next)
        // anchor stays so repeated shift-clicks extend from the same origin
        return
      }
    }
    if (mods.ctrl) {
      const next = new Set(selectedPaths)
      if (next.has(entry.path)) next.delete(entry.path)
      else next.add(entry.path)
      onSelectChange(next)
      setAnchor(entry.path)
      return
    }
    // bare click — single selection
    onSelectChange(new Set([entry.path]))
    setAnchor(entry.path)
  }

  return (
    <section className="panel">
      <header className="panel-head">
        <span className="panel-title">{title}</span>
        <PathBreadcrumb path={path} onNavigate={onNavigate} history={history} />
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
          <FileEntryRow key={e.path} entry={e} selected={selectedPaths.has(e.path)}
            onSelect={(mods) => handleSelect(e, mods)} onOpen={() => onOpen(e.name)}
            onPreview={onPreview ? () => onPreview(e) : undefined}
            onContextMenu={(ev) => onRowContextMenu(e, ev)} />
        ))}
      </div>
    </section>
  )
}
