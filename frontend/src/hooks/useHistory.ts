import { useState, useCallback } from 'react'

// History cap: keep at most this many entries per pane (most recent first).
const HISTORY_LIMIT = 20

// useHistory owns a deduped, capped, localStorage-backed list of visited
// paths for one pane. push() moves `p` to the front (dropping any earlier
// copy) and trims to HISTORY_LIMIT. Storage errors are swallowed so a bad
// entry can never break navigation.
//
// The local and device panes each instantiate this with a distinct storage
// key (`androidfs:hist:local` / `androidfs:hist:device`) so their histories
// stay independent.
export function useHistory(storageKey: string) {
  const [history, setHistory] = useState<string[]>(() => {
    try {
      const raw = localStorage.getItem(storageKey)
      if (!raw) return []
      const v = JSON.parse(raw)
      return Array.isArray(v) ? v.filter(x => typeof x === 'string').slice(0, HISTORY_LIMIT) : []
    } catch {
      return []
    }
  })

  const push = useCallback((p: string) => {
    if (!p) return
    setHistory(prev => {
      // De-dup: drop an existing copy of p, then prepend. Trim to the cap.
      const next = [p, ...prev.filter(x => x !== p)].slice(0, HISTORY_LIMIT)
      try { localStorage.setItem(storageKey, JSON.stringify(next)) } catch { /* ignore */ }
      return next
    })
  }, [storageKey])

  return { history, push }
}
