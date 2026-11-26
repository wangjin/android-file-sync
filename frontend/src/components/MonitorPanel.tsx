import { MonitorSample } from '../hooks/useMonitor'

const W = 100
const H = 40
// Mini per-core charts: same width, shorter viewBox, no grid lines.
const MINI_W = 100
const MINI_H = 24

function buildSegments(values: (number | null)[], w = W, h = H): string[] {
  const n = values.length
  if (n < 2) return []
  const segs: string[] = []
  let cur: string[] = []
  for (let i = 0; i < n; i++) {
    const v = values[i]
    if (v == null) {
      if (cur.length > 1) segs.push(cur.join(' '))
      cur = []
      continue
    }
    const x = (i / (n - 1)) * w
    const y = h - (Math.min(v, 100) / 100) * h
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

// One per-core cell: label, current value and a compact trend line.
function MiniSpark({ label, values, color }: {
  label: string
  values: (number | null)[]
  color: string
}) {
  const latest = [...values].reverse().find(v => v != null)
  const segs = buildSegments(values, MINI_W, MINI_H)

  return (
    <div className="monitor-core-cell">
      <div className="monitor-core-head">
        <span className="monitor-core-label">{label}</span>
        <span className="monitor-core-value mono" style={{ color }}>
          {latest != null ? `${latest.toFixed(1)}%` : '—'}
        </span>
      </div>
      <svg className="monitor-core-chart" viewBox={`0 0 ${MINI_W} ${MINI_H}`} preserveAspectRatio="none" aria-hidden>
        {segs.map((pts, i) => (
          <polyline
            key={i}
            points={pts}
            fill="none"
            stroke={color}
            strokeWidth="1"
            vectorEffect="non-scaling-stroke"
            strokeLinejoin="round"
            strokeLinecap="round"
          />
        ))}
      </svg>
    </div>
  )
}

// Per-core mini-grid below the overall sparkline. The core count is the
// longest cores[] array seen in the visible window (device-independent,
// R5): later frames with fewer cores pad the trailing cells with null → "—".
// When no sample ever carried per-core data the grid is not rendered at all.
function CoreGrid({ samples, field, labelPrefix, color }: {
  samples: MonitorSample[]
  field: 'cpuCores' | 'npuCores'
  labelPrefix: string
  color: string
}) {
  let count = 0
  for (const s of samples) {
    const len = (s[field] ?? []).length
    if (len > count) count = len
  }
  if (count === 0) return null

  const cells = []
  for (let i = 0; i < count; i++) {
    cells.push(
      <MiniSpark
        key={i}
        label={`${labelPrefix}${i}`}
        values={samples.map(s => s[field]?.[i] ?? null)}
        color={color}
      />,
    )
  }
  return <div className="monitor-core-grid">{cells}</div>
}

// The two live sparklines (CPU mint, NPU amber), each followed by a
// per-core mini-grid when the device reports per-core data. Rendered by
// the monitor module's board; the tab chip in the module bar shows the
// overall readout only.
export function MonitorCharts({ samples }: { samples: MonitorSample[] }) {
  return (
    <div className="monitor-body">
      <div className="monitor-group">
        <Sparkline label="CPU" values={samples.map(s => s.cpu)} color="var(--signal)" />
        <CoreGrid samples={samples} field="cpuCores" labelPrefix="cpu" color="var(--signal)" />
      </div>
      <div className="monitor-group">
        <Sparkline label="NPU" values={samples.map(s => s.npu)} color="var(--amber)" />
        <CoreGrid samples={samples} field="npuCores" labelPrefix="Core" color="var(--amber)" />
      </div>
    </div>
  )
}
