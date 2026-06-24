import { useState, useEffect, useRef } from 'react'
import { Events } from '@wailsio/runtime'
import { CheckUpdate } from '../../bindings/androidfs/app.js'
import { Info as UpdateInfo } from '../../bindings/androidfs/internal/update/models.js'

// useUpdate subscribes to the backend's startup auto-check (update:available)
// and exposes a manual checkNow(). Toast messages are surfaced for manual
// feedback ("已是最新版本" / error); the auto-check is silent on failure.
export function useUpdate() {
  const [info, setInfo] = useState<UpdateInfo | null>(null)
  const [toast, setToast] = useState<string | null>(null)
  const timer = useRef<number | null>(null)

  const flash = (msg: string) => {
    setToast(msg)
    if (timer.current) window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setToast(null), 3000)
  }

  // auto-check result from backend
  useEffect(() => {
    const off = Events.On('update:available', (ev: any) => {
      if (ev.data) setInfo(ev.data as UpdateInfo)
    })
    return () => { off() }
  }, [])

  // manual check: always gives feedback
  const checkNow = async () => {
    try {
      const res = await CheckUpdate()
      if (res && res.has_update) {
        setInfo(res)
      } else {
        flash('已是最新版本')
      }
    } catch (e: any) {
      flash('检查更新失败，请检查网络')
    }
  }

  const dismissToast = () => setToast(null)
  const dismissInfo = () => setInfo(null)

  return { info, toast, dismissToast, dismissInfo, checkNow }
}
