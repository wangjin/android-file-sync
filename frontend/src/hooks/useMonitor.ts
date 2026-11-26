import { useState, useEffect, useRef, useCallback } from 'react'
import { Events } from '@wailsio/runtime'

export interface MonitorSample {
  ts: number
  cpu: number | null
  cpuCores?: number[] | null
  npu: number | null
  npuCores?: number[] | null
}

const MAX_SAMPLES = 120
const DEFAULT_INTERVAL = 1000

export function useMonitor() {
  const [running, setRunning] = useState(false)
  const [samples, setSamples] = useState<MonitorSample[]>([])
  const [intervalMs, setIntervalMs] = useState(1000)
  const serialRef = useRef<string | null>(null)

  useEffect(() => {
    const cancel = Events.On('monitor:sample', (ev: any) => {
      const data = ev.data as MonitorSample
      if (!data) return
      setSamples(prev => {
        const next = [...prev, data]
        if (next.length > MAX_SAMPLES) next.shift()
        return next
      })
    })
    return () => { cancel() }
  }, [])

  const start = useCallback(async (serial: string) => {
    serialRef.current = serial
    setSamples([])
    setRunning(true)
    try {
      const { StartMonitor } = await import('../../bindings/androidfs/app.js')
      await StartMonitor(serial, intervalMs)
    } catch (e) {
      console.error('monitor start failed:', e)
      setRunning(false)
    }
  }, [intervalMs])

  const stop = useCallback(() => {
    setRunning(false)
    setSamples([])
    serialRef.current = null
    import('../../bindings/androidfs/app.js').then(({ StopMonitor }) => StopMonitor())
  }, [])

  const changeInterval = useCallback((ms: number) => {
    setIntervalMs(ms)
    if (serialRef.current) {
      import('../../bindings/androidfs/app.js').then(({ StartMonitor }) =>
        StartMonitor(serialRef.current!, ms),
      )
    }
  }, [])

  return { running, samples, intervalMs, changeInterval, start, stop }
}
