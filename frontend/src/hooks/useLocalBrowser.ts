import { useState, useCallback } from 'react'

// Local-pane browsing uses the browser's drag/drop + host paths returned by
// transfers; listing the local FS is handled by the OS via native dialogs and
// drag-drop. This hook tracks a chosen root path the user drags from.
export interface LocalEntry { name: string; path: string; isDir: boolean }

export function useLocalBrowser() {
  const [root, setRoot] = useState<string>('')
  const [staged, setStaged] = useState<LocalEntry[]>([])

  const addStaged = useCallback((e: LocalEntry) => {
    setStaged(prev => prev.some(x => x.path === e.path) ? prev : [...prev, e])
  }, [])
  const clearStaged = useCallback(() => setStaged([]), [])

  return { root, setRoot, staged, addStaged, clearStaged }
}
