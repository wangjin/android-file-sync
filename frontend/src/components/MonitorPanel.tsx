import { MonitorSample } from '../hooks/useMonitor'

const W = 100
const H = 40

function buildSegments(values: (number | null)[]): string[] {
  const n = values.length
  if (n < 2) return []
  const step = W / 120
  const segs: string[] = []
  let cur: string[] = []
  for (let i = 0; i < n; i++) {
    const v = values[i]
    if (v == null) {
      if (cur.length > 1) segs.push(cur.join(' '))
      cur = []
      continue
    }
    const x = (i / (n - 1)) * W
    const y = H - (Math.min(v, 100) / 100) * H
    cur.push(`${x.toFixed(1)},${y.toFixed(1)}`)
  }
  if (cur.length > 1) segs.push(cur.join(' '))
  return segs
}

function Sparkline({ label, values, color }: {
  label: string
  values: (number | null)[]
  color: string
}) {
  const latest = [...values].reverse().find(v => v != null)
  const segs = buildSegments(values)

  return (
    <div className="monitor-metric">
      <div className="monitor-metric-head">
        <span className="monitor-label">{label}</span>
        <span className="monitor-value mono" style={{ color }}>
          {latest != null ? `${latest.toFixed(1)}%` : '—'}
        </span>
      </div>
      <svg className="monitor-chart" viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" aria-hidden>
        {[25, 50, 75].map(p => {
          const y = H - (p / 100) * H
          return <line key={p} x1="0" y1={y} x2={W} y2={y} className="monitor-grid" />
        })}
        {segs.map((pts, i) => (
          <polyline
            key={i}
            points={pts}
            fill="none"
            stroke={color}
            strokeWidth="1.5"
            vectorEffect="non-scaling-stroke"
            strokeLinejoin="round"
            strokeLinecap="round"
          />
        ))}
      </svg>
    </div>
  )
}

export function MonitorPanel({ samples, running, intervalMs, onIntervalChange, onStop }: {
  samples: MonitorSample[]
  running: boolean
  intervalMs: number
  onIntervalChange: (ms: number) => void
  onStop: () => void
}) {
  return (
    <section className="monitor-panel">
      <header className="monitor-head">
        <span className="panel-title monitor-title">
          监控
          {running && <span className="monitor-live-dot" />}
        </span>
        <select
          className="monitor-interval"
          value={intervalMs}
          onChange={e => onIntervalChange(Number(e.target.value))}
          disabled={!running}
        >
          <option value={500}>0.5s</option>
          <option value={1000}>1s</option>
          <option value={2000}>2s</option>
          <option value={5000}>5s</option>
        </select>
        <span className="toolbar-spacer" />
        <button className="telem-cancel" onClick={onStop}>■ 停止</button>
      </header>
      <div className="monitor-body">
        <Sparkline label="CPU" values={samples.map(s => s.cpu)} color="var(--signal)" />
        <Sparkline label="NPU" values={samples.map(s => s.npu)} color="var(--amber)" />
      </div>
    </section>
  )
}
