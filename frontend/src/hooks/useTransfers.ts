import { useState, useEffect, useRef } from 'react'
import { Events } from '@wailsio/runtime'
import { GetTasks } from '../../bindings/androidfs/app.js'
import { TransferTask, TransferState } from '../../bindings/androidfs/internal/model/models.js'

// How long a finished task (done/failed/cancelled) lingers in the telemetry
// panel before auto-dismissing. Transfers-in-flight stay until they finish.
const AUTO_DISMISS_MS = 15_000

function isTerminal(t: TransferTask): boolean {
  return t.state === TransferState.StateDone ||
    t.state === TransferState.StateFailed ||
    t.state === TransferState.StateCancelled
}

export function useTransfers() {
  const [tasks, setTasks] = useState<TransferTask[]>([])
  // Timers keyed by task id, so a manual dismiss can cancel a pending auto one.
  const timers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map())

  const clearTimer = (id: string) => {
    const handle = timers.current.get(id)
    if (handle !== undefined) {
      clearTimeout(handle)
      timers.current.delete(id)
    }
  }

  // dismiss removes a task from the panel (manual close, or auto after the
  // delay). It only touches the local view — the backend queue is unchanged.
  const dismiss = (id: string) => {
    clearTimer(id)
    setTasks(prev => prev.filter(t => t.id !== id))
  }

  // Arm the auto-dismiss timer when a task first becomes terminal. Kept in a
  // ref so the (once-mounted) task:changed subscription can see the latest
  // tasks without re-subscribing on every change.
  const armIfTerminal = useRef((t: TransferTask) => {
    if (!isTerminal(t)) return
    if (timers.current.has(t.id)) return // already armed
    const handle = setTimeout(() => dismiss(t.id), AUTO_DISMISS_MS)
    timers.current.set(t.id, handle)
  })
  armIfTerminal.current = (t: TransferTask) => {
    if (!isTerminal(t)) return
    if (timers.current.has(t.id)) return
    const handle = setTimeout(() => dismiss(t.id), AUTO_DISMISS_MS)
    timers.current.set(t.id, handle)
  }

  useEffect(() => {
    GetTasks().then(t => setTasks((t ?? []).filter((x): x is TransferTask => x !== null)))
    const cancel = Events.On('task:changed', (ev: any) => {
      const t: TransferTask | null = ev.data
      if (!t) return
      setTasks(prev => {
        const i = prev.findIndex(x => x.id === t.id)
        if (i >= 0) { const n = [...prev]; n[i] = t; return n }
        return [...prev, t]
      })
      armIfTerminal.current(t)
    })
    return () => { cancel() }
  }, [])

  // Clear all pending timers on unmount so they can't fire into a dead hook.
  useEffect(() => {
    const map = timers.current
    return () => { map.forEach(h => clearTimeout(h)); map.clear() }
  }, [])

  return { tasks, dismiss }
}
