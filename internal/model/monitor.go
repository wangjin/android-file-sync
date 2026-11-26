package model

// MonitorSample is one point in a device resource-usage time series.
// CPU and NPU are percentages in [0, 100]; a nil pointer means the metric
// was unavailable at that tick (e.g. top produced no parseable line, or the
// rknpu debugfs node does not exist on this SoC). CPUCores/NPUCores carry
// per-core values (0-100) with the slice index equal to the core number;
// nil means per-core data is unavailable (fallback path or unsupported node).
type MonitorSample struct {
	Ts       int64     `json:"ts"`                 // unix milliseconds, host clock
	CPU      *float64  `json:"cpu"`                // overall CPU usage 0-100, nil if unavailable
	CPUCores []float64 `json:"cpuCores,omitempty"` // per-core CPU usage, index = core number
	NPU      *float64  `json:"npu"`                // overall NPU load 0-100 (core mean), nil if node absent
	NPUCores []float64 `json:"npuCores,omitempty"` // per-core NPU load, index = core number
}
