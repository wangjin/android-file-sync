import { useState, useEffect } from 'react'
import { Events } from '@wailsio/runtime'
import { DownloadUpdate } from '../../bindings/androidfs/app.js'
import { Info as UpdateInfo } from '../../bindings/androidfs/internal/update/models.js'

// UpdateDialog shows when an update is available. Clicking 立即下载 starts an
// in-app download with a live progress bar; on completion the backend opens the
// installer and emits update:done. Errors surface inline with a retry.
export function UpdateDialog({ info, onClose }: {
  info: UpdateInfo
  onClose: () => void
}) {
  const [phase, setPhase] = useState<'prompt' | 'downloading' | 'error'>('prompt')
  const [percent, setPercent] = useState(0)
  const [error, setError] = useState('')

  useEffect(() => {
    const offP = Events.On('update:progress', (ev: any) => {
      setPercent(ev.data?.percent ?? 0)
    })
    const offD = Events.On('update:done', () => {
      onClose()   // installer opened; close the dialog
    })
    const offE = Events.On('update:error', (ev: any) => {
      setError(ev.data?.message ?? '下载失败')
      setPhase('error')
    })
    return () => { offP(); offD(); offE() }
  }, [onClose])

  const start = async () => {
    setPhase('downloading')
    setPercent(0)
    await DownloadUpdate(info.download_url)
  }

  return (
    <div className="overlay" role="dialog" aria-modal="true">
      <div className="dialog">
        <h3 className="dialog-title">发现新版本</h3>
        {phase === 'prompt' && (
          <>
            <div className="confirm-message">
              当前 <span className="mono">{info.current_version}</span>，最新
              <span className="update-version mono"> {info.latest_version}</span>。
            </div>
            {info.release_notes && <div className="update-notes">{info.release_notes}</div>}
            <div className="dialog-actions">
              <button onClick={onClose}>稍后</button>
              <button
                className="primary"
                disabled={!info.download_url}
                onClick={start}
              >
                {info.download_url ? '立即下载' : '当前平台暂无安装包'}
              </button>
            </div>
          </>
        )}
        {phase === 'downloading' && (
          <>
            <div className="confirm-message">正在下载…</div>
            <div className="update-progress-track">
              <div className="update-progress-fill" style={{ width: `${percent}%` }} />
            </div>
            <div className="update-pct mono">{percent}%</div>
          </>
        )}
        {phase === 'error' && (
          <>
            <div className="confirm-message">下载失败</div>
            <div className="update-error">{error}</div>
            <div className="dialog-actions">
              <button onClick={onClose}>关闭</button>
              <button className="primary" disabled={!info.download_url} onClick={start}>重试</button>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
