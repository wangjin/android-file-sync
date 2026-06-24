import { useState, useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { GetDevices } from '../../bindings/androidfs/app.js'
import { Device } from '../../bindings/androidfs/internal/model/models.js'

export function useDevices() {
  const [devices, setDevices] = useState<Device[]>([])
  useEffect(() => {
    GetDevices().then(setDevices)
    const cancel = Events.On('device:changed', (ev: any) => {
      setDevices(ev.data?.devices ?? [])
    })
    return () => { cancel() }
  }, [])
  return { devices }
}
