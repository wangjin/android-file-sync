import type { ReactNode } from 'react'

export function EmptyState({ message, action }: { message: string; action?: ReactNode }) {
  return (
    <div className="empty">
      <div className="empty-msg">{message}</div>
      {action}
    </div>
  )
}
