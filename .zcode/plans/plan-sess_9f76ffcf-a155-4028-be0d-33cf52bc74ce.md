# 批量多选功能实现计划

## 目标
列表支持 Ctrl/Cmd+点击切换多选 + Shift+点击范围选择；批量推送/拉取、批量删除走多选；重命名、图片预览保持单选语义。

## 关键发现（决定方案的事实）
- 后端 `PushFiles(serial, localPaths: string[], remoteDir)` / `PullFiles(serial, remotePaths: string[], localDir)` **已接收数组** —— 批量传输零后端改动。
- 后端 `Delete` / `DeleteLocal` 是单路径 —— 批量删除在前端循环。
- 现有 `.row-selected` 样式是纯类名驱动（`FileEntryRow.tsx:15`），天然支持多行同时选中，无需改 CSS 主体。

## 分层设计（职责清晰）
- **FileEntryRow**：只负责上报修饰键 `onSelect(entry, { ctrl, shift })`。
- **FilePanel**：持有 `anchor`，基于 sorted entries 计算选中集合语义（单选/Ctrl 切换/Shift 范围），对外暴露 `onSelectChange(Set)`。
- **App**：持有 `Set<string>` 选中状态，基于 size 驱动批量操作与菜单。

## 改动清单（按文件）

### 1. `frontend/src/components/FileEntryRow.tsx`
- props：`onSelect: (entry: FileEntry, mods: { ctrl: boolean; shift: boolean }) => void`
- `onClick={e => onSelect(entry, { ctrl: e.metaKey || e.ctrlKey, shift: e.shiftKey })}`
- 双击、右键保持不变（双击打开/预览；右键上报不变）。
- `selected: boolean` 判定不变（由父层传入）。

### 2. `frontend/src/components/FilePanel.tsx`
- props 调整：
  - `selectedPath: string | null` → `selectedPaths: Set<string>`
  - `onSelect: (p: string) => void` → `onSelectChange: (paths: Set<string>) => void`
- 内部新增 `const [anchor, setAnchor] = useState<string | null>(null)`。
- 选中逻辑（传给 FileEntryRow 的 onSelect）：
  - **无修饰键（single）**：`new Set([entry.path])`，anchor = entry.path
  - **Ctrl/Cmd（toggle）**：复制当前 set，存在则删、不存在则加；anchor = entry.path
  - **Shift（range）**：若 anchor 且在同一 sorted 列表内，取 anchor↔entry 之间所有 path 与当前 set **并集**（标准 Finder 行为，保留已选）；无 anchor 则退化为 single
  - 计算完调 `onSelectChange(next)` 和 `setAnchor(...)`
- 行渲染：`selected={selectedPaths.has(e.path)}`。

### 3. `frontend/src/App.tsx`（核心改动）
- 状态：`useState<Set<string>>(new Set())` × 2（`localSelected`/`deviceSelected`），`const` 重命名为 `Set` 语义但保留原名以减少 diff。
- 导航时清空选中：包装 `handleNavigate(side, p)` → 先 `setLocal/DeviceSelected(new Set())` 再调底层 `navigate`。两个 `<FilePanel onNavigate={...}>` 指向包装版。
- `serial` 变化时清空 device 选中（已有 `useEffect([serial])`，追加一行）。
- **批量传输**：
  - `pushSelected`：`const paths = [...localSelected]; if (!serial || paths.length === 0) return; await PushFiles(serial, paths, device.path)`
  - `pullSelected`：同理用 `[...deviceSelected]`。
- **批量删除**：
  - `requestDelete` 适配多选：当菜单触发时记录 `{ side, paths: string[], count }`（右键项所在的 pane 当前选中集）。`pendingDelete` 结构改为 `{ side, paths: string[] }`。
  - 确认框文案：`count === 1` 显示原名；`count > 1` 显示"确定删除 N 个选中项？此操作不可撤销。"（用已有 `ConfirmDialog`，message 是 ReactNode）。
  - `confirmDelete`：`for (const p of paths) { await (side==='local'?DeleteLocal(p):Delete(serial,p)) }`，逐个执行，失败用已有 `Toast` 提示单个失败并继续；全部完成后清空该 pane 选中集并 `refresh()`。
- **右键菜单（openMenu + buildMenuItems）**：
  - `openMenu`：若右键项 path 不在当前选中集 → 设为唯一选中（Finder 行为）；若已在 → 保持多选。再 `setMenu({ side, entry, ... })`。
  - `buildMenuItems(side)`：读 `side==='local'?localSelected:deviceSelected` 的 size。
    - size === 1：保留全部菜单项（推送/拉取、重命名、预览[图片]、删除）。
    - size > 1：只显示 `↑ 推送 N 项至设备` / `↓ 拉取 N 项到本地`、`删除 N 项`（隐藏重命名、预览）。
  - 删除项 onSelect 调 `requestDelete(side)`（内部读当前选中集）。
- `<FilePanel>` 调用点 × 2：`selectedPath=` → `selectedPaths=`，`onSelect={setX}` → `onSelectChange={setX}`，`onNavigate={x.navigate}` → `onNavigate={(p)=>handleNavigate('local',p)}`。

### 4. `ConfirmDialog` 调用处（App.tsx 内）
- `message` 依据 `pendingDelete.paths.length` 分支：单选显示 `<strong>{name}</strong>`；多选显示数量文案。
- 保留 `confirmLabel="删除"`。

### 5. `useLocalBrowser.ts` / `useDeviceBrowser.ts`
- **不改**。选中清理由 App 层 `handleNavigate` 包装负责，保持 hooks 纯粹。

## 不在范围内（保持单选语义）
- 重命名（`PromptDialog`）—— 只对单选；多选时菜单不出现该项。
- 图片预览（`ImagePreview`）—— 只对单选；多选时菜单不出现该项。
- 批量重命名、跨 pane 选中 —— 不做。

## 边界处理
- 双击会先触发两次 click（第一次单选清空多选）—— 与 Finder 行为一致，可接受。
- Shift 范围选择若 anchor 已失效（导航/刷新后）—— FilePanel 内部 anchor 在组件卸载或 entries 变化时重置（用 useEffect 清 anchor）。
- 切换目录、切换设备 → 清空对应 pane 选中集。
- task:done 刷新 → 不清空（用户可能继续操作）；删除成功后清空。

## 验证
1. `cd frontend && npm run build`（含 tsc 类型检查）—— 确认 props 签名变更无类型错误。
2. 手动验证场景：
   - 单击单选 → 推送/拉取/删除/重命名/预览 全可用（与现状一致）
   - Cmd/Ctrl+点击 多个 → 菜单显示"推送 N 项""删除 N 项"，重命名/预览消失
   - Shift+点击 → 范围全选
   - 批量删除 → 确认框显示数量，确认后逐个删除并刷新
   - 批量推送/拉取 → 传输面板出现 N 条任务
   - 右键已选中项 → 不清空多选；右键未选中项 → 变为单选该项
   - 切换目录 → 选中集清空

## 风险
- 主要风险是 props 签名变更波及 `FilePanel`/`FileEntryRow` 两个组件 + App 调用点。已用 codegraph 确认调用面：`FileEntryRow` 仅被 `FilePanel` 调用，`FilePanel` 仅被 `App.tsx` 调用，无其他引用，改动封闭。
