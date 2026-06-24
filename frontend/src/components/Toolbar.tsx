import { Device } from '../../bindings/androidfs/internal/model/models.js'

export function Toolbar({ devices, selected, onSelect, onRefresh, onConnect }: {
  devices: Device[]
  selected: string | null
  onSelect: (s: string) => void
  onRefresh: () => void
  onConnect: () => void
}) {
  const warn = selected && devices.find(d => d.serial === selected)?.state === 'unauthorized'
  return (
    <header className="toolbar">
      <span className="brand mono">AndroidFS</span>
      <select className="device-select" value={selected ?? ''} onChange={e => onSelect(e.target.value)}>
        <option value="" disabled>选择设备…</option>
        {devices.map(d => (
          <option key={d.serial} value={d.serial}>
            {d.model || d.serial} · {d.transport} · {d.state}
          </option>
        ))}
      </select>
      {warn && <span className="warn mono">在设备上允许 USB 调试授权</span>}
      <span className="toolbar-spacer" />
      <button onClick={onRefresh}>刷新</button>
      <button onClick={onConnect}>无线连接</button>
    </header>
  )
}
