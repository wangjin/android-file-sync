import { useState, useEffect, useRef } from 'react'

export function PathBreadcrumb({ path, onNavigate, history }: {
  path: string
  onNavigate: (p: string) => void
  history?: string[]
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(path)
  const [showHist, setShowHist] = useState(false)
  const wrapRef = useRef<HTMLDivElement>(null)
  useEffect(() => setDraft(path), [path])

  // Close the history dropdown on any click outside it.
  useEffect(() => {
    if (!showHist) return
    const onDown = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) setShowHist(false)
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [showHist])

  const submit = () => { setEditing(false); if (draft) onNavigate(draft) }

  if (editing) {
    return (
      <input
        className="path-input mono"
        autoFocus
        value={draft}
        onChange={e => setDraft(e.target.value)}
        onBlur={submit}
        onKeyDown={e => e.key === 'Enter' && submit()}
      />
    )
  }
  const parts = path.split('/').filter(Boolean)
  return (
    <div className="crumbs-wrap" ref={wrapRef}>
      <div className="crumbs mono" onClick={() => setEditing(true)}>
        <span className="crumb" onClick={() => onNavigate('/')}>/</span>
        {parts.map((part, i) => {
          const target = '/' + parts.slice(0, i + 1).join('/')
          return (
            <span key={target} className="crumb-seg">
              <span className="crumb" onClick={() => onNavigate(target)}>{part}</span>
              {i < parts.length - 1 ? <span className="crumb-sep">/</span> : null}
            </span>
          )
        })}
      </div>
      {/* History dropdown — only when there are recent paths to show. */}
      {history && history.length > 0 && (
        <button
          className="hist-toggle"
          title="历史路径"
          onClick={e => { e.stopPropagation(); setShowHist(v => !v) }}
        >▼</button>
      )}
      {showHist && history && history.length > 0 && (
        <div className="hist-dropdown mono">
          {history.map(p => (
            <div
              key={p}
              className={'hist-item' + (p === path ? ' active' : '')}
              title={p}
              onClick={() => { setShowHist(false); onNavigate(p) }}
            >{p}</div>
          ))}
        </div>
      )}
    </div>
  )
}
