## 端侧图片预览功能

### 目标
在本地和设备两个面板中，双击图片文件即可在应用内直接预览，无需下载/拉取到本地。支持 jpg/png/gif/webp/bmp/svg。

### 设计要点
- **原图直读 + 大小上限**：设备端读取完整字节返回，设 30MB 上限防卡顿。无新 Go 依赖。
- **两面板对称**：本地端用 `os.ReadFile`，设备端用 `adb exec-out cat`（二进制安全，绕过 `adb shell` 的 LF→CRLF 破坏）。
- **触发方式**：双击图片文件（目前双击只进目录；非图片非目录的双击维持现状）。右键菜单对图片加"预览"项。
- **UI**：复用现有 `.overlay`/`.dialog` 遮罩样式，新建 `ImagePreview.tsx`：暗背景 + 居中大图 + 文件名 + 关闭按钮；支持 Esc 关闭、点击遮罩关闭、图片超出视窗时自适应缩放并允许滚动。

---

### 后端改动（Go）

#### 1. `internal/adb/client.go` — 新增二进制安全读取方法
`Shell` 用 `adb shell`，会把 `\n` 转成 `\r\n`，破坏图片字节。新增方法走 `exec-out`（二进制透传）：
```go
// ExecOut runs `adb exec-out <cmd>` and returns raw stdout bytes (binary-safe,
// unlike `adb shell` which translates LF→CRLF). Used to read image bytes via cat.
func (c *AdbClient) ExecOut(ctx context.Context, serial, command string) ([]byte, error)
```
实现：用 `exec.CommandContext(ctx, c.bin, "-s", serial, "exec-out", command...)`，`hideWindow(cmd)`，`cmd.Output()` 取 stdout（Output 在非零退出码时返回 `*ExitError`，stderr 在 `.Stderr`）。按现有 `run` 的错误风格处理。

#### 2. `internal/adb/preview.go` — 新文件，图片判定 + 设备字节读取
```go
package adb

// MaxImageBytes caps how much we pull for an in-app preview to avoid stalling
// the UI on very large device photos. ~30MB covers typical phone photos.
const MaxImageBytes = 30 * 1024 * 1024

// IsImagePath reports whether a path looks like a previewable image by extension.
func IsImagePath(p string) bool   // 转小写，匹配 .jpg/.jpeg/.png/.gif/.webp/.bmp/.svg

// ReadImage pulls up to MaxImageBytes of a device file's raw bytes via
// `adb exec-out cat <path>`. Returns (bytes, mime, error). mime is inferred
// from the extension. An error is returned if the file exceeds MaxImageBytes so
// the frontend can show a friendly message instead of a binary blob.
func (c *AdbClient) ReadImage(ctx context.Context, serial, path string) ([]byte, string, error)
```
- `IsImagePath`：放这里以便前端也能通过单独 binding 查询（或前端自带同款判定，见下）。
- `ReadImage`：先 `RemoteSize` 查大小，超限返回 `fmt.Errorf("图片过大 (%s)，已超过 %dMB 预览上限", humanSize, MaxImageBytes/MB)`；否则 `ExecOut(ctx, serial, "cat "+quoteArg(path))`。mime 由扩展名映射表给出（`image/jpeg` 等），未知扩展名默认 `application/octet-stream`。
- 错误处理：cat 对不存在文件会输出错误到 stderr 且退出码非 0 → `ExecOut` 返回 `*exec.ExitError`，直接包装返回。

#### 3. `app_local.go` — 新增本地图片读取方法
```go
// ReadLocalImage reads up to MaxImageBytes of a host image file's raw bytes.
// Returns (bytes, mime, error). Over-limit files return an error so the
// frontend can show a friendly message.
func (a *App) ReadLocalImage(path string) ([]byte, string, error)
```
- `os.Stat` 查大小 → 超限（引用 `adb.MaxImageBytes`）返回错误；否则 `os.ReadFile`，mime 用 `mime.TypeByExtension(filepath.Ext(path))`，回退 `application/octet-stream`。
- 返回 `([]byte, string, error)`。Wails v3 会把 `[]byte` 序列化给前端（通过 RPC），前端拿到可直接用于 Blob/Uint8Array。

#### 4. `app_browse.go` — 新增设备图片读取方法
```go
// ReadImage pulls a device image's raw bytes for in-app preview (no download).
// Returns (bytes, mime, error). Files over MaxImageBytes are rejected.
func (a *App) ReadImage(serial, path string) ([]byte, string, error)
```
转发 `a.client.ReadImage(a.ctx, serial, path)`。

> 命名注意：两个方法名不同（`ReadImage` 设备 / `ReadLocalImage` 本地），与现有 `ListDir`/`ListLocalDir`、`Delete`/`DeleteLocal` 命名对称一致。

---

### 前端改动（TS/React）

#### 5. 重新生成 bindings
运行 `wails3 generate bindings -ts -d frontend/bindings .`，产出 `ReadImage` / `ReadLocalImage` 两个新函数到 `frontend/bindings/androidfs/app.ts`（同现有 `PullFiles` 等风格）。

#### 6. `frontend/src/lib/image.ts` — 新文件，图片判定 + 大小展示工具
```ts
const IMG_EXT = new Set(['jpg','jpeg','png','gif','webp','bmp','svg'])
export function isImageFile(entry: FileEntry): boolean  // 非目录 && 扩展名命中
export function humanSize(n: number): string            // 复用 FileEntryRow 的格式逻辑
```
前端自带判定，避免为每个 row 发 binding 调用；与 Go 端 `IsImagePath` 保持扩展名集合一致（注释互相指明）。

#### 7. `frontend/src/components/ImagePreview.tsx` — 新组件
- Props: `{ entry: FileEntry; onClose: () => void }`。
- 用 `useState` 持 `url`/`loading`/`error`。
- `useEffect`：根据 entry.path 来源（设备需 serial）调用 `ReadImage(serial, path)` 或 `ReadLocalImage(path)`，把返回的 `Uint8Array` 包成 `new Blob([bytes], { type: mime })` → `URL.createObjectURL` → 设 url。组件卸载时 `URL.revokeObjectURL`。
- 渲染：`.overlay` 暗遮罩；内部容器放 `<img>` + 顶部文件名条 + 右上 × 关闭按钮 + loading/error 占位。Esc 键关闭（`useEffect` 加 keydown 监听）。点遮罩（非图片本体）关闭。
- CSS：新建 `.image-preview`、`.image-preview-frame`、`.image-preview-bar`、`.image-preview-img`、`.image-preview-spinner`、`.image-preview-error`（追加到 `style.css` 末尾）。图片 `max-width:90vw; max-height:80vh; object-fit:contain`。

#### 8. `frontend/src/App.tsx` — 接线
- 新增 state: `const [preview, setPreview] = useState<FileEntry | null>(null)`。
- 新增 `openPreview(entry: FileEntry)`：`if (!isImageFile(entry)) return; setPreview(entry)`。
- **双击预览**：改 `FilePanel` 调用处，给 `onOpen` 传一个函数——当 entry 是图片时预览，否则维持原"进目录"逻辑。由于 `FilePanel.onOpen` 当前只接 `name`（目录用），需要把图片双击处理提到 App 层：给 `FileEntryRow` 的双击在 FilePanel 里区分——**更小改动**：在 `App.tsx` 里新增 `onOpenImage(entry)` 回调传给 `FilePanel`，`FilePanel` 把它转给 `FileEntryRow` 的 `onDoubleClick`（图片用），保留原 `onOpen`（目录）。
  - 具体：`FilePanel` props 增 `onOpenImage?: (entry: FileEntry) => void`；`FileEntryRow` props 增 `onDoubleClickImage?: () => void`；`FileEntryRow` 的根 div 加 `onDoubleClick={entry.is_dir ? onOpen : (onDoubleClickImage ?? undefined)}`。
- **右键菜单预览**：`buildMenuItems` 里，对图片 entry 在两项首部插入 `{ label: '👁 预览', onSelect: () => openPreview(entry) }`。
- 在 JSX 末尾（Toast 前）渲染：`{preview && <ImagePreview entry={preview} onClose={() => setPreview(null)} />}`。
- 顶部 `EmptyState` 提示文案补一句"双击图片可直接预览"。

---

### 验证清单
1. `go build ./...` 通过（`go vet ./...` 干净）。
2. `wails3 generate bindings` 重新生成后，`frontend/bindings/androidfs/app.ts` 含 `ReadImage`/`ReadLocalImage`。
3. `npm --prefix frontend run build` 通过（tsc 类型检查 + vite 构建）。
4. `go test ./internal/...`：现有用例不回归；为 `IsImagePath`、`ReadImage` 大小上限、`ExecOut` 错误路径补单元测试（`IsImagePath` 纯函数表驱动；`ExecOut`/`ReadImage` 用现有 fake binary 风格，参考 `internal/adb/transfer_test.go`）。
5. 手动验收（由用户做）：连设备 → 进入含图片的目录 → 双击 jpg → 预览出现、图片清晰、Esc/点遮罩/× 关闭正常；本地面板同理；超大文件给友好错误。

### 风险与回退
- `adb exec-out cat` 在极旧 platform-tools 上可能不支持 → 回退用 `adb pull` 到临时文件再读。本计划先用 exec-out；若用户反馈旧设备失败，再加 pull 回退（独立改动）。
- Wails v3 `[]byte` 返回值的序列化：v3 通过 `Call.ByID` 返回的字节数组前端拿到的是 `Uint8Array`/`number[]`，用 `new Blob([new Uint8Array(...)])` 兼容。实现时按实际返回类型微调包装代码。