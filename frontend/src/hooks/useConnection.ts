import { useState, useCallback } from 'react'
import { ConnectDevice } from '../../bindings/androidfs/app.js'

export function useConnection() {
  const [connecting, setConnecting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const connect = useCallback(async (addr: string) => {
    setConnecting(true); setError(null)
    try {
      await ConnectDevice(addr)
    } catch (e: any) {
      setError(e?.message ?? String(e))
    } finally {
      setConnecting(false)
    }
  }, [])

  return { connecting, error, connect }
}
