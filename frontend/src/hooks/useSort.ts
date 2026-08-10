import { useState, useCallback } from 'react'
import type { FileEntry } from '../../bindings/androidfs/internal/model/models.js'

export type SortKey = 'name' | 'size' | 'time'
export type SortDir = 'asc' | 'desc'

export interface SortState { key: SortKey; dir: SortDir }

const SORT_KEY = 'androidfs:sort'
const DEFAULT_SORT: SortState = { key: 'name', dir: 'asc' }

// The sort preference is global (shared by both panes) and persisted across
// launches via localStorage. Invalid/corrupt stored values fall back to the
// default so a bad entry can never break rendering.
function loadSort(): SortState {
  try {
    const raw = localStorage.getItem(SORT_KEY)
    if (!raw) return DEFAULT_SORT
    const v = JSON.parse(raw) as Partial<SortState>
    const keys: SortKey[] = ['name', 'size', 'time']
    const dirs: SortDir[] = ['asc', 'desc']
    if (keys.includes(v.key as SortKey) && dirs.includes(v.dir as SortDir)) {
      return { key: v.key as SortKey, dir: v.dir as SortDir }
    }
  } catch {
    // ignore parse / storage errors — fall back to default
  }
  return DEFAULT_SORT
}

function saveSort(s: SortState) {
  try { localStorage.setItem(SORT_KEY, JSON.stringify(s)) } catch { /* ignore */ }
}

// useSort owns the sort selection for one pane and exposes a toggle that
// cycles: same key -> flip dir; different key -> set key, default asc. The
// selection is read from localStorage on mount and written back on every
// change, so the user's sort habit survives restarts.
export function useSort(): { sort: SortState; toggle: (key: SortKey) => void } {
  const [sort, setSort] = useState<SortState>(loadSort)
  const toggle = useCallback((key: SortKey) => {
    setSort(prev => {
      const next: SortState = prev.key === key
        ? { key, dir: prev.dir === 'asc' ? 'desc' : 'asc' }
        : { key, dir: 'asc' }
      saveSort(next)
      return next
    })
  }, [])
  return { sort, toggle }
}

// sortEntries returns a new array sorted by the given state. Directories always
// come before files (the standard file-manager convention); within each group
// entries are ordered by the chosen key and direction.
export function sortEntries(entries: FileEntry[], sort: SortState): FileEntry[] {
  const cmp = compare(sort)
  return [...entries].sort((a, b) => {
    if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
    return cmp(a, b)
  })
}

function compare(sort: SortState): (a: FileEntry, b: FileEntry) => number {
  const mul = sort.dir === 'asc' ? 1 : -1
  switch (sort.key) {
    case 'size':
      return (a, b) => (a.size - b.size) * mul
    case 'time': {
      return (a, b) => {
        const at = a.mod_time ? Date.parse(a.mod_time as any) : 0
        const bt = b.mod_time ? Date.parse(b.mod_time as any) : 0
        return (at - bt) * mul
      }
    }
    case 'name':
    default:
      return (a, b) => a.name.localeCompare(b.name, undefined, { numeric: true }) * mul
  }
}
