import { useState, useCallback } from 'react'
import type { FileEntry } from '../../bindings/androidfs/internal/model/models.js'

export type SortKey = 'name' | 'size' | 'time'
export type SortDir = 'asc' | 'desc'

export interface SortState { key: SortKey; dir: SortDir }

// useSort owns the sort selection for one pane and exposes a toggle that
// cycles: same key -> flip dir; different key -> set key, default asc.
export function useSort(initial: SortState = { key: 'name', dir: 'asc' }) {
  const [sort, setSort] = useState<SortState>(initial)
  const toggle = useCallback((key: SortKey) => {
    setSort(prev => prev.key === key
      ? { key, dir: prev.dir === 'asc' ? 'desc' : 'asc' }
      : { key, dir: 'asc' })
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
