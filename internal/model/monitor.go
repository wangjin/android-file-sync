package model

// MonitorSample is one point in a device resource-usage time series.
// CPU and NPU are percentages in [0, 100]; a nil pointer means the metric
// was unavailable at that tick (e.g. top produced no parseable line, or the
// rknpu debugfs node does not exist on this SoC).
type MonitorSample struct {
	Ts  int64    `json:"ts"`  // unix milliseconds, host clock
	CPU *float64 `json:"cpu"` // overall CPU usage 0-100, nil if unavailable
	NPU *float64 `json:"npu"` // NPU load 0-100, nil if node absent / unparsable
}
