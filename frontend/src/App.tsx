import { useEffect, useState } from 'react'
import { GetDevices } from '../bindings/androidfs/app.js'
import { Events } from '@wailsio/runtime'
import { Device } from '../bindings/androidfs/internal/model/models.js'

export default function App() {
  const [devices, setDevices] = useState<Device[]>([])

  useEffect(() => {
    GetDevices().then(setDevices).catch(console.error)
    Events.On('device:changed', (ev: any) => {
      setDevices(ev.data?.devices ?? [])
    })
  }, [])

  return (
    <div className="app-shell">
      <h1 className="app-title mono">AndroidFS</h1>
      <p className="muted">设备: {devices.length}</p>
      <ul>
        {devices.map(d => (
          <li key={d.serial} className="mono">{d.model || d.serial} — {d.state}</li>
        ))}
      </ul>
    </div>
  )
}
