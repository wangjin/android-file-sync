# PRD: 批量 UX 增强（5 项）

## Goal

一次合入 5 项互相独立、范围明确的体验改进，提升 Android 文件浏览器的日常使用舒适度：
复制文件名、记住排序习惯、地址栏历史、刷新联动本地、升级下载加速前缀。

## Background / Confirmed Facts（来自代码）

- 右键菜单在 `frontend/src/App.tsx:174-200` 的 `buildMenuItems(side, entry)` 构建；`MenuItem` 形状为 `{ label, onSelect, disabled?, danger? }`（`frontend/src/components/ContextMenu.tsx`）。「重命名」当前是 `single`-only（仅 count===1 时出现），是 single-only 菜单项的现成范式。
- 排序状态由 `frontend/src/hooks/useSort.ts` 的 `useSort(initial?)` 管理，在 `frontend/src/components/FilePanel.tsx:27` 以默认值 `{ key:'name', dir:'asc' }` 实例化。全仓库当前**没有**任何 `localStorage` 使用。
- 地址栏是 `frontend/src/components/PathBreadcrumb.tsx`：可点击面包屑 + 点击进入编辑态（一个 `<input>`）。两端面板各自渲染一个。无历史/下拉。
- Toolbar 的刷新回调在 `App.tsx:210`：`onRefresh={() => (serial ? device.refresh() : location.reload())}` —— **只刷设备面板或整页 reload，本地面板 `local.refresh()` 不会被调用**。
- 升级代理前缀在 `internal/update/update.go:15`：`const ProxyPrefix = "https://ghproxy.homeboyc.cn/"`，被 `withProxy()` 拼接到 GitHub URL 前，用于 release 检查（`Check`）和资源下载（`Download`）两个路径。受影响测试：`internal/update/update_test.go:12-16` 的 `TestWithProxy` 写死了旧前缀。

## Requirements

### R1. 右键「复制文件名」（single-only）
- 在 `buildMenuItems` 中新增「📋 复制文件名」菜单项，**仅当 `single === true`（count===1）时**显示，与「重命名」并列。
- 点击后把 `entry.name`（仅文件名，不含路径）写入系统剪贴板。
- 实现优先级：浏览器 `navigator.clipboard.writeText`（Wails webview 支持）；失败时静默 fallback 不报错弹窗。
- 菜单项位置：放在「重命名」之前或之后，紧邻即可；不依赖设备连接状态（`disabled` 不基于 `serial`）。

### R2. 记住排序习惯（localStorage 持久化）
- 排序状态 `{ key, dir }` 持久化到 `localStorage`，key 例如 `androidfs:sort`。
- 启动时读取；读取失败/无值时回退默认 `{ key:'name', dir:'asc' }`。
- 排序变化时写入（去抖不必须，排序点击频率低）。
- 作用范围：**全局共享一份排序**（两个面板共用同一 key），符合「默认排序习惯」语义，面板间通常期望一致，实现也最简。
- 改造点：`useSort` 内部读取/写入 localStorage，hook 自管，无需 App 层透传。

### R3. 地址栏历史（本地+设备各自，各 20 条）
- 本地面板、设备面板各自维护一份历史，key 分别为 `androidfs:hist:local`、`androidfs:hist:device`。
- `navigate(p)` 成功进入一个新目录后，把 `p` 追加到历史（去重，最新的在前，截断保留前 20 条）。
- UI：在 `PathBreadcrumb` 的面包屑右侧加一个 `▼` 下拉按钮，点击展开最近路径列表，选中后调用 `onNavigate`。下拉用轻量浮层（绝对定位），点击外部关闭。
- 历史为空时不显示 `▼`。
- 仅持久化字符串路径，不含其它数据。

### R4. 刷新按钮联动本地列表
- Toolbar「刷新」点击时，**同时刷新**设备面板与本地面板，而不是只刷一边 / 整页 reload。
- 已连接设备：`device.refresh()` + `local.refresh()`。
- 未连接设备（`!serial`）：仍然刷新本地（`local.refresh()`），不再 `location.reload()` 整页（避免丢失状态；未连接时也没有设备列表需要 reload 刷新）。
- 设备变更（`serial` 切换）走原有 `useEffect`，不受影响。

### R5. 升级下载加速前缀改为 `https://ghfast.top/`
- `internal/update/update.go` 的 `ProxyPrefix` 常量值由 `https://ghproxy.homeboyc.cn/` 改为 `https://ghfast.top/`。
- 同步更新 `internal/update/update_test.go:13` 中 `TestWithProxy` 的期望值。
- 其余逻辑（先代理后直连、release 检查与下载共用前缀）保持不变。

## Acceptance Criteria

- **AC1** 单选一个文件 → 右键出现「复制文件名」→ 点击后粘贴得到该文件名（不含路径）；多选时不出现该项。
- **AC2** 在某面板按名称/大小/修改时间排序并切换升降序 → 关闭应用重新打开 → 排序状态与关闭前一致；两个面板共用同一排序习惯。
- **AC3** 本地面板进入若干目录后，地址栏右侧出现 `▼`，点开列出最近访问的路径（去重、最新在前、≤20 条）；选中后跳转该目录。设备面板同理且与本地面板历史相互独立。历史为空时无 `▼`。
- **AC4** 已连接设备状态下点「刷新」→ 本地与设备两个面板的文件列表都重新加载（loading 态可见）；未连接设备时点「刷新」→ 本地面板刷新，页面不整页 reload。
- **AC5** `go test ./internal/update/...` 全绿，`TestWithProxy` 期望值更新为 `https://ghfast.top/...`；手动触发「检查更新」流程时 release 检查与（若有更新）下载均走 `https://ghfast.top/` 前缀。
- **AC6** `cd frontend && npm run build`（tsc + vite build）无类型错误。

## Out of Scope

- 复制完整路径（仅复制文件名；完整路径是未来可选）。
- 多选拼接复制多个文件名。
- 排序习惯按面板分别记忆（本次为全局共享）。
- 地址栏历史的编辑/删除/置顶等管理功能（仅自动追加+去重+截断）。
- 刷新时清空选择或重置排序（刷新只重新拉列表，不动选择/排序）。
- 升级流程的其它改动（多镜像、手动选镜像、断点续传等）。

## Technical Notes / 改动点清单

| # | 文件 | 改动 |
|---|------|------|
| R1 | `frontend/src/App.tsx` (`buildMenuItems`) | single 分支内加 `{ label: '📋 复制文件名', onSelect: () => copyName(entry) }`；新增 `copyName` 用 `navigator.clipboard.writeText` |
| R2 | `frontend/src/hooks/useSort.ts` | `useSort` 初始值从 `localStorage` 读、`setSort` 后写回；封装读写 helper |
| R3 | `frontend/src/hooks/useLocalBrowser.ts`, `frontend/src/hooks/useDeviceBrowser.ts` | navigate 成功后写历史；暴露 `history` |
| R3 | `frontend/src/components/PathBreadcrumb.tsx` | 接收 `history?: string[]`，渲染 `▼` 下拉 |
| R3 | `frontend/src/components/FilePanel.tsx`, `frontend/src/App.tsx` | 透传 history 到 PathBreadcrumb |
| R4 | `frontend/src/App.tsx`（Toolbar `onRefresh`） | 改为同时调 `local.refresh()` 和（有 serial 时）`device.refresh()` |
| R5 | `internal/update/update.go`, `internal/update/update_test.go` | `ProxyPrefix` 值与测试期望 |

## Risks / Deferred

- `navigator.clipboard` 在某些 webview 安全上下文下可能不可用 → fallback 静默失败，不阻塞菜单（可接受，后续可加 Toast 提示）。
- `localStorage` 在 Wails webview 持久性依赖底层 webview 行为，通常可靠；若发现不持久再排查。
- 地址栏下拉浮层的点击外部关闭/滚动关闭等细节属实现打磨，不作为验收硬性条件。

## Open Questions

无（全部已由用户决策）。
