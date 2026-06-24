import { useState, useEffect } from 'react'

export function PathBreadcrumb({ path, onNavigate }: {
  path: string
  onNavigate: (p: string) => void
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(path)
  useEffect(() => setDraft(path), [path])

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
  )
}
