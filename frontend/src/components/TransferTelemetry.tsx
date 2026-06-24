import { TransferTask, TransferState, TransferDirection } from '../../bindings/androidfs/internal/model/models.js'
import { CancelTask } from '../../bindings/androidfs/app.js'

// The signature element: mid-seam telemetry. When a task is active it lights
// up --signal and shows live readouts. This is the one memorable thing; the
// rest stays disciplined.
export function TransferTelemetry({ tasks }: { tasks: TransferTask[] }) {
  if (tasks.length === 0) return null
  return (
    <section className="telemetry">
      {tasks.map(t => {
        const pct = t.total > 0 ? Math.round((t.bytes / t.total) * 100) : 0
        const active = t.state === TransferState.StateActive
        const terminal = t.state === TransferState.StateDone ||
          t.state === TransferState.StateFailed ||
          t.state === TransferState.StateCancelled
        return (
          <div key={t.id} className={['telem-row', active ? 'telem-active' : ''].join(' ')}>
            <span className="telem-dir mono">
              {t.direction === TransferDirection.DirPush ? '↑ push' : '↓ pull'}
            </span>
            <span className="telem-name">{t.file_name}</span>
            <span className="telem-rate mono">
              {active ? `${(t.speed / 1024 / 1024).toFixed(1)} MB/s` : stateLabel(t.state)}
            </span>
            <div className="telem-track" aria-hidden>
              <div
                className={['telem-fill', active ? 'telem-flow' : ''].join(' ')}
                style={{ width: `${pct}%` }}
              />
            </div>
            <span className="telem-pct mono">{pct}%</span>
            {!terminal && (
              <button className="telem-cancel" onClick={() => CancelTask(t.id)}>取消</button>
            )}
          </div>
        )
      })}
    </section>
  )
}

function stateLabel(state: TransferState): string {
  switch (state) {
    case TransferState.StateDone: return 'done'
    case TransferState.StateFailed: return 'failed'
    case TransferState.StateCancelled: return 'cancelled'
    case TransferState.StateActive: return 'active'
    default: return 'pending'
  }
}
