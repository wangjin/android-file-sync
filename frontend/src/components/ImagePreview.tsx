import { useEffect, useState } from 'react'
import { FileEntry } from '../../bindings/androidfs/internal/model/models.js'
import { ReadImage, ReadLocalImage } from '../../bindings/androidfs/app.js'
import { decodeBase64Bytes, humanSize } from '../lib/image'

// ImagePreview fetches an image's raw bytes from the backend (device via
// adb exec-out, or local via os.ReadFile) and renders it in a lightbox overlay
// WITHOUT downloading to disk. The blob lives in memory only and is revoked on
// close. Esc, clicking the backdrop, or the × button dismisses it.
export function ImagePreview({ entry, side, serial, onClose }: {
  entry: FileEntry
  side: 'local' | 'device'
  serial: string | null
  onClose: () => void
}) {
  const [url, setUrl] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let revoked = false
    let madeUrl: string | null = null
    setLoading(true); setError(null); setUrl(null)

    const p = side === 'local'
      ? ReadLocalImage(entry.path)
      : serial
        ? ReadImage(serial, entry.path)
        : Promise.reject(new Error('未连接设备'))

    p.then((res: any) => {
      // Wails returns (bytes, mime); bytes arrive as base64 over JSON RPC.
      const bytes = decodeBase64Bytes(res?.[0])
      const mime = (typeof res?.[1] === 'string' && res[1]) || 'application/octet-stream'
      const blob = new Blob([bytes as BlobPart], { type: mime })
      madeUrl = URL.createObjectURL(blob)
      if (!revoked) { setUrl(madeUrl); setLoading(false) }
    }).catch((e: any) => {
      if (!revoked) { setError(e?.message ?? String(e)); setLoading(false) }
    })

    return () => {
      revoked = true
      if (madeUrl) URL.revokeObjectURL(madeUrl)
    }
  }, [entry.path, side, serial])

  // Esc to close.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div className="overlay image-preview" role="dialog" aria-modal="true" onClick={onClose}>
      <div className="image-preview-bar" onClick={(e) => e.stopPropagation()}>
        <span className="image-preview-name" title={entry.path}>{entry.name}</span>
        <span className="image-preview-meta mono">{humanSize(entry.size)}</span>
        <button className="image-preview-close" onClick={onClose} aria-label="关闭">✕</button>
      </div>
      <div className="image-preview-frame" onClick={(e) => e.stopPropagation()}>
        {loading && <div className="image-preview-spinner muted">加载中…</div>}
        {error && <div className="image-preview-error">无法预览：{error}</div>}
        {url && !loading && !error && (
          <img className="image-preview-img" src={url} alt={entry.name} />
        )}
      </div>
    </div>
  )
}
