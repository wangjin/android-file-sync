import { useState, useCallback } from 'react'
import { ListDir } from '../../bindings/androidfs/app.js'
import { FileEntry } from '../../bindings/androidfs/internal/model/models.js'

export function useDeviceBrowser(serial: string | null) {
  const [path, setPath] = useState('/')
  const [entries, setEntries] = useState<FileEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async (p = path) => {
    if (!serial) return
    setLoading(true); setError(null)
    try {
      const list = await ListDir(serial, p)
      setEntries(list ?? [])
      setPath(p)
    } catch (e: any) {
      setError(e?.message ?? String(e))
    } finally {
      setLoading(false)
    }
  }, [serial, path])

  const navigate = useCallback((p: string) => refresh(p), [refresh])
  const enter = useCallback((name: string) => navigate(`${path}/${name}`.replace(/\/+/g, '/')), [navigate, path])

  return { path, entries, loading, error, refresh, navigate, enter }
}
