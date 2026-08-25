export type ModuleId = 'files' | 'monitor'

export interface MonitorReadout {
  cpu: number | null
  npu: number | null
}

// The module switcher: top-level views that are peers of each other
// (文件 = browse + transfer, 监控 = CPU/NPU sampling). The monitor tab
// carries a live readout chip while sampling is active, so the data is
// still visible from the files view without stealing layout space.
export function ModuleBar({ module, onChange, monitorActive, monitorLatest }: {
  module: ModuleId
  onChange: (m: ModuleId) => void
  monitorActive: boolean
  monitorLatest: MonitorReadout | null
}) {
  return (
    <nav className="module-bar">
      <button
        className={['module-tab', module === 'files' ? 'module-tab-active' : ''].join(' ')}
        onClick={() => onChange('files')}
      >
        <span className="module-tab-icon" aria-hidden>📁</span>
        文件
      </button>
      <button
        className={['module-tab', module === 'monitor' ? 'module-tab-active' : ''].join(' ')}
        onClick={() => onChange('monitor')}
      >
        <span className="module-tab-icon" aria-hidden>📈</span>
        监控
        {monitorActive && monitorLatest && (
          <span className="module-tab-live mono">
            <span className="monitor-live-dot" aria-hidden />
            {monitorLatest.cpu != null ? `CPU ${monitorLatest.cpu.toFixed(1)}%` : 'CPU —'}
            <span className="module-tab-live-sep" aria-hidden>·</span>
            {monitorLatest.npu != null ? `NPU ${monitorLatest.npu.toFixed(1)}%` : 'NPU —'}
          </span>
        )}
      </button>
    </nav>
  )
}
