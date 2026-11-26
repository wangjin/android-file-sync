# Device Metric Sampling Guidelines（设备资源采集）

> 适用于 `internal/adb/monitor.go` 及一切通过 adb shell 读取设备实时指标的代码。

---

## 核心约束

### 1. 一次采样 = 一次 adb 连接
`monitor:sample` 由固定间隔的 ticker 驱动（`app_monitor.go`）。**每次采样的 adb 命令连接数不得增加**——多开连接会叠加 adb 往返延迟，间隔拉到 200ms 时会饿死 ticker。

需要"两个时刻的快照"做差分时（见下），必须在**单条 shell 命令内**完成：
```
cat /proc/stat; echo '<marker>'; sleep <S>; cat /proc/stat
```
sleep 放在命令里，而不是 Go 侧调两次 `c.Shell`。

### 2. 瞬时值来自差分，不能读单帧
`/proc/stat` 的 `cpu` / `cpuN` 行是**累计 jiffies**，单帧只是开机以来的总量，不是使用率。每核/总体使用率必须由两帧差分：
```
busy  = Δtotal - (Δidle + Δiowait)
usage = 100 * busy / Δtotal        （clampPct 到 [0,100]）
```
- sleep 窗口 S 由采样间隔推出（`cpuSampleSleep`：`interval/3` 夹到 `[100ms,1s]`），保证 S 不会接近 interval。
- 采样超时 = `5s + S`，把命令内 sleep 预留出来。
- 用带引号的 marker（`echo '###...###'`）拆分两帧：`/proc/stat` 是多行文本，直拼无法确定性切分；marker 不加引号时开头的 `#` 会被 shell 当注释吞掉。

### 3. 差分用 float64，别用无符号相减
jiffies 是 `uint64`。两帧之间计数器若被重置（重启/热插拔），无符号相减会**回绕成一个巨大正数**，悄悄通过 `Δ>0` 检查。先转 `float64` 再相减。

### 4. 按核索引对齐，不接受"空洞"
两帧的核顺序/数量不保证一致（热插拔、输出截断）。按 `cpuN` 的数字 N 建 map 对齐（`parseProcStatFrame`），聚合行 `cpu` 用索引 `-1`。核数组要求 `cpu0..cpuN` 连续且全部可差分；任一核 Δtotal=0（两帧读到相同计数，即该核空闲到没有 tick）或数据缺失 → 整组 per-core 置 nil，但总体仍可由聚合行给出。

### 5. NPU：每核明细 + 总体取"平均"
`/sys/kernel/debug/rknpu/load` 打印 `Core0: x%, Core1: y%, Core2: z%`。
- 用 `core\s*(\d+)\s*:\s*(\d+...)` 同时捕获**核索引和值**——别只抓值，`Core0/1/2` 里的数字会被误当成负载（这正是旧版踩过的坑）。
- 总体语义 = **各核平均**（不是 max）。旧版取 max 会在单核任务时高估整体，是有意的语义修正。
- 无 `CoreN` 标签的设备（单值/裸数字）回退标量语义：cores=nil，总体取百分比 max。

### 6. 全程 best-effort，nil 即降级
所有采集失败（命令报错、节点缺失、解析失败）一律让对应指标为 nil → 前端显示 `—`，绝不 panic 或让整块面板空白。`top` 总体解析（`parseTopCPU`）保留为 `/proc/stat` 不可用时的兜底路径。

---

## 跨层数据契约

`MonitorSample`（`internal/model/monitor.go`）通过 `monitor:sample` 事件直达前端。新增字段是**纯增量**（`cpuCores`/`npuCores` + `omitempty`）：

- `MonitorSample` 不出现在任何 Wails 服务方法签名里，只是事件负载 → `wails3 generate bindings` 不会为它生成 TS 类型。前端 TS 契约的唯一来源是 `frontend/src/hooks/useMonitor.ts` 的手写 `interface`，改字段时**两处手动保持同步**。
- 核数不写死（不同 SoC 核数不同）。前端按当前窗口内出现过的最大数组长度渲染，尾格补 `—`（`CoreGrid`）。

---

## 测试基线

`internal/adb/monitor_test.go` 覆盖：procps/toybox/RK3588 三种 `top` 总体格式、`/proc/stat` 两帧差分（8 核 / Δtotal=0 空洞 / 缺列 / 乱序+CRLF / 无聚合行）、marker 拆帧、NPU 真机样例。改解析逻辑必须同步更新这些用例，不要删旧用例——它们锁住兜底路径与格式兼容。
