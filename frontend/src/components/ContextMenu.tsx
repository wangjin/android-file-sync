import { useEffect, useRef } from 'react'

export interface MenuItem {
  label: string
  onSelect: () => void
  disabled?: boolean
  danger?: boolean
}

// ContextMenu renders at the cursor position and closes on outside click,
// escape, or scroll. It is a controlled component: the parent owns open state
// and the item list.
export function ContextMenu({ x, y, items, onClose }: {
  x: number
  y: number
  items: MenuItem[]
  onClose: () => void
}) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    const onScroll = () => onClose()
    window.addEventListener('mousedown', onDown)
    window.addEventListener('keydown', onKey)
    window.addEventListener('blur', onClose)
    window.addEventListener('wheel', onScroll, { passive: true })
    return () => {
      window.removeEventListener('mousedown', onDown)
      window.removeEventListener('keydown', onKey)
      window.removeEventListener('blur', onClose)
      window.removeEventListener('wheel', onScroll)
    }
  }, [onClose])

  return (
    <div
      ref={ref}
      className="context-menu"
      style={{ left: x, top: y }}
      role="menu"
    >
      {items.map((item, i) => (
        <button
          key={i}
          role="menuitem"
          className={['context-item', item.danger ? 'context-danger' : ''].join(' ')}
          disabled={item.disabled}
          onClick={() => { item.onSelect(); onClose() }}
        >
          {item.label}
        </button>
      ))}
    </div>
  )
}
