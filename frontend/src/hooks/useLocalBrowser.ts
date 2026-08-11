import { useState, useCallback, useEffect } from 'react'
import { ListLocalDir, HomePath } from '../../bindings/androidfs/app.js'
import { FileEntry } from '../../bindings/androidfs/internal/model/models.js'
import { useHistory } from './useHistory'

// useLocalBrowser drives the host-side (local) pane the same way
// useDeviceBrowser drives the device pane: it lists a directory and exposes
// navigation. The root defaults to the user's home directory.
export function useLocalBrowser() {
  const [root, setRoot] = useState<string>('')
  const [entries, setEntries] = useState<FileEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const { history, push: pushHistory, clear: clearHistory } = useHistory('androidfs:hist:local')

  // Default to the host home directory on first mount.
  useEffect(() => {
    HomePath().then(h => refresh(h)).catch(() => {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const refresh = useCallback(async (p = root) => {
    if (!p) return
    setLoading(true); setError(null)
    try {
      const list = await ListLocalDir(p)
      setEntries(list ?? [])
      setRoot(p)
      pushHistory(p)   // only record paths we actually listed successfully
    } catch (e: any) {
      setError(e?.message ?? String(e))
      setEntries([])
    } finally {
      setLoading(false)
    }
  }, [root, pushHistory])

  const navigate = useCallback((p: string) => refresh(p), [refresh])
  const enter = useCallback((name: string) => {
    // Avoid relying on posix join in the browser; the backend joins paths.
    const next = root.endsWith('/') ? root + name : root + '/' + name
    navigate(next)
  }, [navigate, root])

  return { root, setRoot, entries, loading, error, refresh, navigate, enter, history, clearHistory }
}
