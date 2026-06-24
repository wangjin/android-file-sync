import { useState, useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { GetTasks } from '../../bindings/androidfs/app.js'
import { TransferTask } from '../../bindings/androidfs/internal/model/models.js'

export function useTransfers() {
  const [tasks, setTasks] = useState<TransferTask[]>([])
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
    })
    return () => { cancel() }
  }, [])
  return { tasks }
}
