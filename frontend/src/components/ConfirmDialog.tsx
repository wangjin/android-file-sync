import type { ReactNode } from 'react'

// ConfirmDialog is a generic two-step confirmation, used for destructive
// actions like delete. The confirm button is vermilion (danger) to make the
// consequence unmistakable. Returns null when closed so callers can keep it
// mounted unconditionally.
export function ConfirmDialog({ open, title, message, confirmLabel, onConfirm, onCancel }: {
  open: boolean
  title: string
  message: ReactNode
  confirmLabel: string
  onConfirm: () => void
  onCancel: () => void
}) {
  if (!open) return null
  return (
    <div className="overlay" role="dialog" aria-modal="true">
      <div className="dialog">
        <h3 className="dialog-title">{title}</h3>
        <div className="confirm-message">{message}</div>
        <div className="dialog-actions">
          <button onClick={onCancel}>取消</button>
          <button className="danger" onClick={onConfirm}>{confirmLabel}</button>
        </div>
      </div>
    </div>
  )
}
