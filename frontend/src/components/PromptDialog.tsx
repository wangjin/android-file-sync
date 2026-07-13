import { useState, useEffect, useRef } from 'react'

// PromptDialog is a generic single-line input dialog (the prompt analogue of
// ConfirmDialog). It mirrors the .overlay/.dialog shell the other dialogs use.
// Each time it opens the field is reset to initialValue, focused, and its text
// selected so the user can type over it immediately. Enter confirms, Escape
// cancels. An empty (whitespace-only) value disables the confirm button.
export function PromptDialog({ open, title, initialValue, confirmLabel, onConfirm, onCancel }: {
  open: boolean
  title: string
  initialValue: string
  confirmLabel: string
  onConfirm: (value: string) => void
  onCancel: () => void
}) {
  const [value, setValue] = useState(initialValue)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (open) {
      setValue(initialValue)
      // Focus + select on next tick so the input is mounted before we touch it.
      const id = requestAnimationFrame(() => {
        inputRef.current?.focus()
        inputRef.current?.select()
      })
      return () => cancelAnimationFrame(id)
    }
  }, [open, initialValue])

  if (!open) return null

  const trimmed = value.trim()
  const submit = () => { if (trimmed) onConfirm(trimmed) }

  return (
    <div className="overlay" role="dialog" aria-modal="true">
      <div className="dialog">
        <h3 className="dialog-title">{title}</h3>
        <input
          ref={inputRef}
          className="mono dialog-input"
          value={value}
          onChange={e => setValue(e.target.value)}
          onKeyDown={e => {
            if (e.key === 'Enter') { e.preventDefault(); submit() }
            else if (e.key === 'Escape') { e.preventDefault(); onCancel() }
          }}
        />
        <div className="dialog-actions">
          <button onClick={onCancel}>取消</button>
          <button className="primary" disabled={!trimmed} onClick={submit}>{confirmLabel}</button>
        </div>
      </div>
    </div>
  )
}
