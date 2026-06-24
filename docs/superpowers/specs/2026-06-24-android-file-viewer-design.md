# Android File Viewer — 设计文档

- **日期**: 2026-06-24
- **状态**: 已批准
- **作者**: wangjin
- **参考项目**: `local-file-share` (Nearfy) — 复用其 Wails3 + Go + React 模式与 CI 流水线

## 1. 概述

一个跨平台(macOS / Windows / Linux)桌面 GUI 工具,通过 ADB 协议查看并管理 Android 设备的文件系统,支持读(浏览)和写(上传/下载/文件操作)。

### 目标

- 浏览设备文件系统(含 SD 卡 `/sdcard`,root 设备可访问全盘 `/`)
- 设备 ↔ 电脑双向文件传输(push / pull)
- 文件管理:新建文件夹、重命名、删除
- 双栏界面,本地与设备并排,支持拖拽互传
- USB 与无线(tcpip)两种连接方式
- 内置 adb 二进制,开箱即用,无需用户另装 SDK

### 非目标 (YAGNI)

- 不做 MTP 协议支持(后期可扩展,本次不做)
- 不做设备端 App(纯 ADB 客户端)
- 不做设备投屏 / 应用管理 / 截屏等 ADB 其它能力
- 不做云端同步

## 2. 需求决策汇总

| 维度 | 决策 |
|------|------|
| 形态 | 桌面 GUI 应用 (Wails3 + Web) |
| 语言/框架 | Go 后端 + React/TS 前端 (Vite) |
| 设备通信 | ADB 协议,调用内置 adb 二进制 |
| 连接方式 | USB + 无线(tcpip) |
| 访问范围 | 全文件系统(设备已 root 可访问 `/`;非 root 受限 `/sdcard` 等) |
| 功能 | 浏览目录、上传/下载、文件管理、双栏互传 |
| 列目录方式 | 解析 `adb shell ls -la` 文本输出 |
| 传输进度 | 解析 `adb push/pull` 写到 stderr 的百分比行 |
| 路径输入 | 设备面板含自由路径输入框,可跳任意路径 |

## 3. 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│  前端 (React + TS, Vite)  —  对齐 Nearfy: components/hooks   │
│  ┌────────────────┐  ┌────────────────┐  ┌───────────────┐  │
│  │ 本地文件面板    │  │ 设备文件面板    │  │ 工具栏/状态栏  │  │
│  └───────┬────────┘  └───────┬────────┘  └───────┬───────┘  │
│          └──────────┬────────┘                   │          │
│        bindings/*.ts 自动生成方法调用              │          │
│        Events.On('xxx') 监听后端推送              │          │
└─────────────────────┬────────────────────────────┘──────────┘
                      │
┌─────────────────────┴────────────────────────────────────────┐
│  后端 (Go, Wails v3 application)                             │
│                                                               │
│  app.go            App struct + ServiceStartup/Shutdown        │
│  app_device.go     设备发现/连接(USB/无线)                      │
│  app_browse.go     列目录/列文件/路径导航                       │
│  app_transfer.go   push/pull + 进度回调 + 任务队列              │
│  app_fileops.go    mkdir/rename/delete                         │
│  main.go           application.New + Window + embed assets     │
│                                                               │
│  internal/                                                     │
│    adb/        AdbClient: exec 内置 adb 二进制,解析输出          │
│    transfer/   传输引擎 + 进度回调 + 任务队列                     │
│    model/      Device / FileEntry / TransferTask 结构体         │
└────────────────────────────────────────────────────────────────┘
                      │
┌─────────────────────┴────────────────────────────────────────┐
│  内置 adb 二进制 (各平台 platform-tools,~5MB/平台)             │
└────────────────────────────────────────────────────────────────┘
```

### 分层职责

- **前端**:纯 UI 与交互,通过自动生成的 `bindings/*.ts` 调后端方法,通过 `Events.On(...)` 订阅后端推送的事件。不直接执行文件/进程操作。
- **命令层(`app_*.go`)**:Wails3 服务方法,定义对外业务接口(参数 / 返回类型),编排调度,处理事件推送。复用 Nearfy 的 `App` 服务注册模式(`application.NewService` + `ServiceStartup/Shutdown`)。
- **适配器层(`internal/adb`、`internal/transfer`)**:封装 adb 二进制调用与传输引擎的具体实现。命令层不关心细节。
- **二进制分发(`platform-tools` 嵌入)**:`//go:embed` + 平台 build tag,运行时解压到缓存目录。

## 4. ADB 适配层(核心)

### 4.1 包结构

```
internal/adb/
├── client.go       # AdbClient: 定位二进制 + exec 命令
├── device.go       # Device 结构体 + ListDevices
├── browse.go       # ListDir: adb shell ls -la 解析
├── transfer.go     # Push/Pull: 进度回调 + context cancel
├── fileops.go      # Mkdir/Rename/Delete/Stat
└── shell.go        # runShell(): 统一执行 adb shell <cmd>
```

### 4.2 AdbClient — 二进制定位与执行

- **首次启动**:从内置资源解压对应平台的 platform-tools 到缓存目录
  - macOS: `~/Library/Caches/AndroidFS/platform-tools/adb`
  - Windows: `%LOCALAPPDATA%\AndroidFS\platform-tools\adb.exe` (含 `AdbWinApi.dll`、`AdbWinUsbApi.dll`)
  - Linux: `~/.cache/AndroidFS/platform-tools/adb`
- **后续调用**:所有命令通过该绝对路径执行,不依赖系统 `PATH`。
- **嵌入方式**:`//go:embed` 各平台 zip 进二进制;打包时用 Wails build tag (`GOOS`/`GOARCH`) 只嵌入目标平台一份。
- **启动自检**:`ServiceStartup` 中执行 `adb version` 验证可用,失败则向前端推送错误事件。

### 4.3 设备发现与连接

- **USB 设备(自动)**:`adb devices -l` → 解析序列号 / 型号 / 状态。
- **无线连接**:前端输入 `ip:port`,后端执行 `adb connect <ip:port>`,成功后加入设备列表。
- **设备变化事件**:后台 goroutine 每 2 秒轮询 `adb devices`,diff 出增/删/状态变化后 `Emit("device:changed", ...)`,前端实时刷新。
- **设备状态机**:`offline → unauthorized → device`。对 `unauthorized`(设备未授权 USB 调试)给出明确前端提示,而非当作"无设备"。

### 4.4 列目录(ListDir)

- 执行 `adb shell ls -la <path>`,在 Go 端用正则解析输出:
  - 权限位(`drwxr-xr-x`)→ 判断 `IsDir` + `Mode`
  - 链接数、owner、group、size、日期、文件名 → 映射成统一 `FileEntry`
- **文件名特殊字符处理**:中文 / 空格 / 特殊符号文件名需正确处理(`ls -la` 输出按列对齐,最后一列为文件名,可能含空格)。
- **符号链接**:解析 `link -> target`,记录到 `FileEntry.Link`。

### 4.5 传输(Push / Pull)

- **命令**:
  - push: `adb push -s <serial> <local> <remote>`
  - pull: `adb pull -s <serial> <remote> <local>`
- **进度**:解析 adb 写到 stderr 的百分比行(如 `1204/4500 (27%)`),逐行读取更新 `TransferTask.Bytes` 与 `Speed`,经事件推送前端。
- **取消**:用 `context` 取消子进程;取消后清理半成品目标文件,任务标记 `cancelled`。
- **队列**:复用 Nearfy 的 `queue.Manager` 并发控制模型(可配置并发数),任务执行体从"自研 TCP"换成"调 adb push/pull"。

## 5. 数据模型

```go
// internal/model/device.go
type Device struct {
    Serial    string // adb 序列号(IP:port 或 USB 序列号)
    State     string // device / offline / unauthorized
    Model     string // 设备型号
    Transport string // usb / tcpip
}

// internal/model/entry.go
type FileEntry struct {
    Name    string
    Path    string    // 完整路径
    IsDir   bool
    Size    int64
    ModTime time.Time
    Mode    string    // 权限位 "drwxr-xr-x"
    Link    string    // 符号链接目标(若有)
}

// internal/model/task.go —— 复用 Nearfy 的 TransferTask
type TransferTask struct {
    ID        string
    Direction string   // "push" / "pull"
    FileName  string
    SrcPath   string
    DstPath   string
    Total     int64
    Bytes     int64    // 已传输
    Speed     float64
    State     string   // pending / active / done / failed / cancelled
    Error     string
}
```

前端对应的 TS 类型由 `bindings/*.ts` 自动生成,无需手写。

## 6. 前端结构

```
frontend/src/
├── App.tsx                      # 主布局:双栏 + 工具栏 + 传输面板
├── main.tsx
├── hooks/
│   ├── useDevices.ts            # 设备列表(USB/无线),监听 device:changed
│   ├── useLocalBrowser.ts       # 本地文件面板:目录导航/列表
│   ├── useDeviceBrowser.ts      # 设备文件面板:列目录/导航
│   ├── useTransfers.ts          # 传输任务队列(移植 Nearfy)
│   └── useConnection.ts         # 无线连接:输入 ip:port → connect
├── components/
│   ├── Toolbar.tsx              # 顶部:设备选择/连接/刷新/路径栏
│   ├── FilePanel.tsx            # 通用双栏面板(本地/设备各一份实例)
│   ├── FileEntryRow.tsx         # 单行:图标/名/大小/日期/权限
│   ├── PathBreadcrumb.tsx       # 路径面包屑导航
│   ├── TransferPanel.tsx        # 底部传输队列(移植 Nearfy)
│   ├── ConnectDialog.tsx        # 无线连接对话框
│   └── EmptyState.tsx
```

### 双栏布局

```
┌──────────────────────────────────────────────────────┐
│ [设备:Pixel 7 ▾] [刷新]  [路径: /sdcard/Download > ] │  ← Toolbar
├────────────────────────┬─────────────────────────────┤
│   本地 (Mac)            │   设备 (Pixel 7)            │
│  /Users/wangjin/        │  /sdcard/                   │
│  📁 Downloads/          │  📁 DCIM/                   │
│  📁 Documents/          │  📁 Download/               │
│  📄 notes.txt           │  📄 backup.db              │
│                        │                             │
│  [↑ 推送到设备]  ←拖拽→  │  [↓ 拉取到本地]            │
├────────────────────────┴─────────────────────────────┤
│ 传输队列:  [████░░] photo.jpg  3.2MB 45%   [取消]    │  ← TransferPanel
└──────────────────────────────────────────────────────┘
```

### 交互流

- **导航**:双击文件夹进入;面包屑跳转;本地面板默认 home,设备面板默认 `/sdcard`。
- **自由路径**:设备面板工具栏含路径输入框,可输入任意路径(如 `/`、`/data`、`/system`)回车跳转。配合 root 访问全文件系统。
- **互传**:选中文件 → 点「推送/拉取」按钮;或**拖拽**跨栏(复用 Nearfy 的 `useDragDrop` + Wails `EnableFileDrop` / `WindowFilesDropped` 事件)。
- **文件管理**:右键菜单 → 新建文件夹 / 重命名 / 删除(调后端 `app_fileops.go`)。

## 7. 错误处理

| 场景 | 处理 |
|------|------|
| 未授权(unauthorized) | 前端提示"请在设备上确认 USB 调试授权",不报错刷屏 |
| adb 二进制启动失败 | 启动时自检,失败弹窗指引 |
| 列目录权限不足 | `permission denied` 行标红,其余正常显示 |
| 传输中断 | 清理半成品文件,任务标 failed,可重试 |
| 设备掉线 | `device:changed` 推送 offline,前端禁用操作 |

所有后端方法返回 `(result, error)`,前端用统一 toast 展示错误,不阻断整个界面。

## 8. 跨平台打包

### 内置 adb(platform-tools)

```
build/adb/darwin-arm64/adb       (M 系列 Mac)
build/adb/darwin-amd64/adb       (Intel Mac)
build/adb/windows-amd64/adb.exe  (+ AdbWinApi.dll, AdbWinUsbApi.dll)
build/adb/linux-amd64/adb
```

- 打包时按 `GOOS`/`GOARCH` 用 build tag 只嵌入目标平台一份,通过 `//go:embed` 进二进制。
- 运行时解压到缓存目录(见 §4.2),解压后赋予可执行权限(macOS/Linux)。

### USB 驱动

- macOS / Linux:系统自带。
- Windows:需用户安装 OEM USB 驱动(Google USB Driver 或厂商驱动)。README 中说明,工具内不处理。

### 技术栈版本(对齐 Nearfy)

- Wails v3 (`v3.0.0-alpha.95`)
- Go:本地 `go.mod` 声明 `go 1.25`;CI 用 `setup-go` 的 `1.26`(高于本地声明的最低版本,提供工具链)。两者不冲突——`go.mod` 的是语言版本下限,CI 用更高版本编译。
- React 18 + TypeScript + Vite
- `@wailsio/runtime`(事件 / 绑定)

## 9. CI/CD(GitHub Actions,复制并适配 Nearfy)

复制 Nearfy 的两条流水线,适配本项目名称 `AndroidFS` 与构建产物路径。CI 通过 Taskfile 驱动,与本地命令一致。

### 9.1 `.github/workflows/build.yml`(PR 测试)

- **触发**:PR 到 `main`
- **job `test`**(ubuntu-latest):
  - checkout + setup-go (1.26) + 安装 Wails Linux 依赖(libgtk-4-dev / libwebkitgtk-6.0-dev / libsoup-3.0-dev)
  - 缓存 `~/go/bin`(Taskfile / wails3 工具)
  - `go install` Taskfile → `task test`

### 9.2 `.github/workflows/release.yml`(发版)

- **触发**:push tag `v*` 或 `workflow_dispatch`
- **permissions**: `contents: write`
- **job `test`**:同 build.yml 的 test。
- **job `build-windows`**(windows-latest,needs test):setup-go + setup-node(22,npm 缓存)+ 安装 wails3 / Taskfile → `task package:windows` → 上传 `build/bin/AndroidFS-windows-amd64.exe`。
- **job `build-macos`**(macos-26,needs test):setup-go + setup-node + 安装 wails3 / Taskfile → `task package:darwin` → 用 `hdiutil` 生成 `AndroidFS-macos.dmg` → 上传。
- **job `release`**(ubuntu-latest,needs [build-windows, build-macos]):下载产物,生成 release notes(按 `feat`/`fix` commit 分类,中文文案),用 `softprops/action-gh-release@v2` 创建 Release,附 Windows exe 与 macOS dmg。

> 与 Nearfy 的差异点:产物 / app / dmg 名称统一改为 `AndroidFS`;release notes 文案中"Nearfy"替换为"AndroidFS"。

### 9.3 `Taskfile.yml`

复制 Nearfy 的 Taskfile 结构,改动:
- `APP_NAME` → `AndroidFS`
- macOS `Info.plist` 中 `CFBundleIdentifier` → `com.ieemoo.androidfs`、`CFBundleExecutable` / 名称 → `AndroidFS`
- dmg `volname` → `AndroidFS`
- 保留:`dev` / `generate`(bindings) / `frontend:install` / `frontend:build` / `build:<os>:<arch>` / `package:<os>` / `test` / `test:coverage` / `clean` / `lint` / `tidy`。
- platform-tools 二进制:**直接提交进仓库** `build/adb/<os>-<arch>/`(参照 Nearfy 把图标等资源提交进 `build/` 的做法),不走运行时下载脚本。仓库随之变大(~5MB/平台 × 4),但保证构建可复现、无外部网络依赖。

## 10. 测试策略

- **Go 单元测试**(`task test`):重点测试 `internal/adb` 的输出解析(`ls -la` 文本 → `FileEntry`、stderr 进度行 → 百分比),用真实 adb 输出样例做 fixture,无需真机。
- **解析稳健性**:覆盖中文/空格/特殊符号文件名、符号链接、permission denied 行。
- 前端不强制单元测试(MVP 阶段),靠手动验证。

## 11. 项目目录结构(最终)

```
android-file-viewer/
├── .github/workflows/
│   ├── build.yml
│   └── release.yml
├── build/
│   ├── adb/
│   │   ├── darwin-arm64/adb
│   │   ├── darwin-amd64/adb
│   │   ├── windows-amd64/adb.exe (+ dll)
│   │   └── linux-amd64/adb
│   └── darwin/                 # Info.plist, icons.icns (生成)
├── docs/superpowers/specs/
├── frontend/
│   ├── src/{hooks,components, ...}
│   ├── bindings/               # 自动生成
│   ├── index.html
│   └── package.json
├── internal/
│   ├── adb/
│   ├── transfer/
│   └── model/
├── app.go
├── app_device.go
├── app_browse.go
├── app_transfer.go
├── app_fileops.go
├── main.go
├── go.mod
├── Taskfile.yml
└── wails.json
```
