# 版本号显示与自动更新功能设计

**日期**: 2026-06-24
**状态**: 已确认，待实现
**GitHub Release 源**: https://github.com/wangjin/android-file-sync
**加速代理**: `https://ghproxy.homeboyc.cn/`（拼接在 GitHub URL 前）

## 1. 背景与目标

AndroidFS 是基于 Wails v3 的桌面应用（Go 后端 + React 前端），已有 GitHub Release 流水线产出 macOS `.dmg` 和 Windows `.exe` 安装包。当前缺少：

- 给用户展示当前运行版本号
- 检测是否有新版本
- 引导用户下载安装新版本

本设计实现：工具栏显示版本号、启动自动检查更新、手动检查、有更新时弹提示、应用内下载（带进度条）、下载完成后自动打开安装包。考虑国内访问 GitHub 受限，所有请求通过 ghproxy 加速，代理失败时回退直连。

### 现有基础

- `main.go` 已有 `var version = "dev"`，构建时通过 `-ldflags "-X main.version=..."` 从 `git describe --tags --always` 注入
- `Taskfile.yml` 的 `build` 系列任务已正确注入版本号
- `.github/workflows/release.yml` 已产出 `AndroidFS-macos.dmg` 和 `AndroidFS-windows-amd64.exe` 两个资产
- `app_runtime.go` 已有 `runtimeGOOS()` 返回平台
- 前端 bindings 通过 `wails3 generate bindings -ts` 自动生成

## 2. 架构：Go 后端驱动（方案 A）

核心决策：更新检查与下载逻辑全部放在 Go 后端。理由：

1. 与项目现有架构一致（`internal/` 包 + `App` 方法 + 自动生成前端 bindings）
2. `net/http` 网络栈成熟，代理/重试/超时控制干净
3. 下载到本地文件 + 用系统命令打开安装包是 Go 的职责（webview 无此权限）
4. 所有更新逻辑集中一处，可测试、可维护

前端为纯 UI 层：调用 App 方法 + 监听 Wails 事件。

### 新增文件结构

```
internal/update/
├── update.go          # 核心逻辑：检查更新、版本比较、资产匹配
├── download.go        # 下载逻辑：带进度、写到临时文件
├── open.go            # 打开安装包（平台分发：open / explorer）
├── version.go         # 版本号解析与比较（semver）
└── *_test.go          # 单元测试（版本比较、资产匹配、代理URL）

app_update.go          # 新增 App 层方法：Version / CheckUpdate / DownloadUpdate
internal/model/update.go  # 新增数据模型 UpdateInfo
```

> `main.go` 无需改动：`var version` 已存在，`App` 服务已注册。`main.version` 作为包级变量被 `app_update.go` 引用。

## 3. 组件与接口

### 数据模型（`internal/model/update.go`）

```go
type UpdateInfo struct {
    CurrentVersion string   // 当前版本，如 "v1.0.0"
    LatestVersion  string   // 最新版本
    HasUpdate      bool     // 是否有更新
    DownloadURL    string   // 对应平台的原始 GitHub 下载直链（NOT 已拼代理）
    ReleaseNotes   string   // 发布说明
}
```

> **代理包装时机**：`DownloadURL` 存储原始 GitHub URL（`https://github.com/.../releases/download/...`）。代理包装发生在请求时——`update.Check` 请求 API 时包装、`update.Download` 下载时包装，而非存入 `DownloadURL`。这样前端展示的链接是干净的，下载时由后端统一决定走代理还是直连。

前端通过自动生成的 binding 直接拿到结构体，无需手写。

### 后端组件职责

| 组件 | 职责 | 对外接口 |
|------|------|---------|
| `update.Parse` / `Compare` | 解析 semver、比较版本高低 | 纯函数 |
| `update.Check(ctx)` | 请求 GitHub API，代理优先+回退，匹配平台资产 | `(*Info, error)` |
| `update.Download(ctx, url, onProgress)` | 下载到临时目录，回调进度 | `(path, error)` |
| `update.Open(path)` | 平台分发打开安装包 | `error` |

### App 层方法（`app_update.go`，暴露给前端）

| 方法 | 说明 |
|------|------|
| `App.Version()` | 返回当前 `main.version` |
| `App.CheckUpdate()` | 调 `update.Check`，返回 `UpdateInfo` |
| `App.DownloadUpdate(url)` | 异步下载，发射进度事件 |

## 4. 数据流

### 流程 A：启动自动检查

```
应用启动 (ServiceStartup)
  └─ 延迟 3s 后（避免抢占启动期网络/adb）
     └─ App.CheckUpdate()
        └─ update.Check(ctx)
           ├─ 解析当前版本（"dev" 时直接跳过，视为无更新）
           ├─ 请求 GitHub API:
           │   1. 优先 https://ghproxy.homeboyc.cn/https://api.github.com/repos/wangjin/android-file-sync/releases/latest
           │   2. 失败（超时 8s/非200）→ 回退 https://api.github.com/repos/.../releases/latest
           │   3. 仍失败 → 返回 error，前端不弹窗（静默失败，不打扰）
           ├─ 解析最新 tag_name，update.Compare(current, latest)
           ├─ 按 runtimeGOOS 匹配资产:
           │     darwin  → AndroidFS-macos.dmg
           │     windows → AndroidFS-windows-amd64.exe
           └─ 返回 UpdateInfo（DownloadURL 存原始 GitHub URL，代理在下载时包装）
        └─ 前端收到后:
           ├─ 有更新 (HasUpdate=true) → 弹更新提示
           └─ 无更新 → 不弹（自动检查静默）
```

### 流程 B：手动检查

用户点击工具栏版本号 → 调 `CheckUpdate()` → 同流程 A，但**无论有无更新都给反馈**（有更新弹窗，无更新显示 toast "已是最新版本"）。区别于自动检查的静默失败。

### 流程 C：下载 + 打开

```
用户在更新提示框点「下载」
  └─ App.DownloadUpdate(info.DownloadURL)
     └─ 启动 goroutine 异步下载（避免阻塞 UI）:
        ├─ update.Download(ctx, url, onProgress)
        │   ├─ 创建临时文件（os.UserCacheDir()/AndroidFS/update.<ext>）
        │   ├─ HTTP GET（带超时、重试 1 次）
        │   ├─ io.Copy 到文件，每 500ms 通过 onProgress 回调:
        │   │     Emit("update:progress", {percent, downloadedBytes, totalBytes})
        │   └─ 返回本地文件路径
        └─ 下载成功:
           ├─ update.Open(path):
           │     darwin  → exec.Command("open", path)
           │     windows → exec.Command("cmd", "/c", "start", "", path)
           └─ 发射事件 update:done
        └─ 下载失败:
           └─ 发射事件 update:error（前端显示错误提示，允许重试）
```

### 事件清单

| 事件名 | 载荷 | 触发场景 |
|--------|------|---------|
| `update:available` | `UpdateInfo` | 自动/手动检查发现有更新 |
| `update:status` | `{message}` | 手动检查时无更新（"已是最新版本"） |
| `update:progress` | `{percent, downloadedBytes, totalBytes}` | 下载过程中周期推送 |
| `update:done` | `{path}` | 下载完成并已打开安装包 |
| `update:error` | `{message}` | 检查/下载失败 |

## 5. 错误处理与边界情况

### 版本比较的边界

GitHub Release 的 `tag_name` 格式不确定（可能是 `v1.0.0`、`1.0.0`、`v1.0`）。

- **解析**：去掉 `v` 前缀，按 `.` 分割为 `major.minor.patch`，缺位补 0（`1.0` → `[1,0,0]`）
- **非 semver**：解析失败时退化为**字符串比较**，并打日志。绝不让版本解析异常导致崩溃。
- **当前版本为 `dev`**：直接返回"无更新"，不报错（开发环境常见）。
- **当前版本含 commit hash**（如 `git describe` 产出 `v1.0.0-3-gabc123`）：只取 `-` 前的部分比较。

### 网络错误处理策略

| 场景 | 自动检查（启动） | 手动检查 |
|------|------------------|---------|
| ghproxy 超时/失败 | 静默回退直连 | 静默回退直连 |
| 直连也失败 | **静默失败，不弹窗** | 弹 toast "检查更新失败，请检查网络" |
| GitHub 返回非 200 | 静默失败 | 弹 toast 错误信息 |
| 无最新 release（404） | 视为无更新 | toast "暂无发布版本" |

原则：**自动检查永远静默失败**（不打扰用户），**手动检查必须给反馈**。

### 下载错误处理

| 场景 | 处理 |
|------|------|
| 代理下载失败 | 回退直连重试 1 次，仍失败则发 `update:error` |
| 磁盘空间不足 | 发 `update:error`，提示"磁盘空间不足" |
| 临时文件写入失败 | 清理半成品，发 `update:error` |
| 用户中途关闭应用 | `ctx.Done()` 取消，清理临时文件 |

### 平台资产缺失

如果 GitHub Release 里**没有**匹配当前平台的资产（例如只有 dmg 没有 exe）：

- 检查阶段：`HasUpdate=true` 但 `DownloadURL` 为空
- 前端：提示框显示"有新版本 vX.X.X"，但下载按钮禁用，说明"当前平台暂无安装包，请前往 GitHub 手动下载"，附 release 页面链接。

### 重试与超时参数

- **HTTP 超时**：检查请求 8s，下载请求 30s（下载用整体 ctx 控制，非单次超时）
- **重试**：检查不重试（代理→直连已是两层）；下载失败重试 1 次
- **进度节流**：进度回调最多每 500ms 一次，避免高频事件淹没前端

### 安全考虑

- ghproxy 前缀硬编码为常量，不接受外部输入拼接（防注入）
- 下载 URL 来自 GitHub Release 的 `browser_download_url`（原始 GitHub URL），下载时由后端按"代理优先+直连回退"策略包装，而非信任前端传入的完整 URL
- 用独立的 `http.Client`，不共享其他连接池

## 6. 测试策略

沿用项目约定 `go test ./internal/... -v -race`。`internal/update` 是纯逻辑层，最适合测试。

### 单元测试范围

#### 1. 版本比较（`version_test.go`）— 最核心，表驱动

```go
TestParse:
  - "v1.0.0"     → [1,0,0], ok=true
  - "1.0.0"      → [1,0,0], ok=true      // 无 v 前缀
  - "1.0"        → [1,0,0], ok=true      // 缺位补 0
  - "v1.0.0-3-gabc" → [1,0,0], ok=true   // git describe 格式
  - "dev"        → ok=false              // 开发版本
  - "garbage"    → ok=false, 不 panic

TestCompare:
  - ("v1.0.0", "v1.0.1")  → -1  (旧)
  - ("v1.1.0", "v1.0.9")  → +1  (新)
  - ("1.0.0", "v1.0.0")   →  0  (相等，忽略前缀)
  - ("v2.0.0", "v1.9.9")  → +1
```

#### 2. 资产匹配（`update_test.go`）

```go
TestMatchAsset:
  - darwin + [AndroidFS-macos.dmg, *.exe] → dmg
  - windows + [*.dmg, AndroidFS-windows-amd64.exe] → exe
  - 无匹配 → ("", false)
```

#### 3. 代理 URL 构造（`update_test.go`）

```go
TestProxyURL:
  - ("https://api.github.com/x", proxy) → "https://ghproxy.../https://api.github.com/x"
  - 下载资产 URL 的包装同理
```

### 用 httptest mock 网络

版本比较/资产匹配是纯函数直接测。检查与下载涉及网络，用 `httptest.NewServer` 起本地服务：

```go
TestCheck:
  - mock 返回合法 release JSON → 正确解析 UpdateInfo
  - mock 返回 404 → error
  - mock 代理失败 + 直连成功 → 回退逻辑生效

TestDownload:
  - mock 返回 100 字节 → 下载完整，进度回调被调用
  - mock 返回 500 → error，重试 1 次
```

### 不做单元测试的部分

- **`App` 层方法**（`app_update.go`）：薄封装，依赖 Wails 的 `application.Get()`，集成测试成本高，靠手动验证
- **`Open()` 打开安装包**：涉及系统调用，靠手动验证
- **前端 UI**：React 组件靠手动验证

### 手动验证清单（实现后）

1. `task dev` 启动，工具栏显示 `AndroidFS v...`
2. 点击版本号 → 手动检查（无 release 时显示"已是最新"或失败 toast）
3. 构造一个比当前版本高的 release → 弹更新提示
4. 点下载 → 进度条推进 → 完成后打开安装包
5. macOS 打开 .dmg / Windows 打开 .exe 验证

## 7. UI 设计（前端）

- **版本号位置**：工具栏品牌名旁，`AndroidFS v1.0.0`。版本号可点击，点击触发手动检查。
- **更新提示弹窗**：复用现有 `ConfirmDialog` 模式。标题"发现新版本"，内容显示新版本号 + 发布说明（截断），按钮"立即下载"/"稍后"。无平台资产时下载按钮禁用并提示。
- **下载进度**：弹窗内或独立浮层显示进度条（percent）。
- **手动检查反馈**：无更新/失败时用轻量 toast，不打断操作。
