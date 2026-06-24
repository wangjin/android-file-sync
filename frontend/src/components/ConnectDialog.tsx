import { useState } from 'react'
import { useConnection } from '../hooks/useConnection'

export function ConnectDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [addr, setAddr] = useState('')
  const { connect, connecting, error } = useConnection()
  if (!open) return null
  return (
    <div className="overlay" role="dialog" aria-modal="true">
      <div className="dialog">
        <h3 className="dialog-title">无线连接</h3>
        <input
          className="mono dialog-input"
          placeholder="192.168.1.20:5555"
          value={addr}
          onChange={e => setAddr(e.target.value)}
        />
        {error && <div className="dialog-error">{error}</div>}
        <div className="dialog-actions">
          <button onClick={onClose}>取消</button>
          <button
            className="primary"
            disabled={connecting || !addr}
            onClick={async () => { await connect(addr); onClose() }}
          >
            {connecting ? '连接中…' : '连接'}
          </button>
        </div>
      </div>
    </div>
  )
}
