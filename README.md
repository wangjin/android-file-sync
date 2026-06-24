# AndroidFS

跨平台(macOS / Windows / Linux)的 Android 设备文件查看器。通过 ADB 浏览并管理 Android 设备的文件系统,支持读写、双向传输。桌面 GUI,开箱即用——首次启动自动下载 adb,无需用户另装 SDK。

![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-blue)
![wails](https://img.shields.io/badge/Wails-v3%20alpha.95-orange)

## 功能

- **双栏文件管理**:本地与设备并排,各自独立导航,支持拖拽互传
- **设备浏览**:列出目录/文件(名称、大小、修改时间、权限),路径面包屑 + 自由路径输入框,设备面板默认从 `/` 起(root 设备可访问全盘)
- **读写传输**:`adb push` / `adb pull`,实时进度(速率、百分比),可取消
- **文件操作**:新建文件夹、重命名、删除
- **连接方式**:USB(需设备开启 USB 调试) + 无线(`adb connect ip:port`)
- **自动获取 adb**:首次启动从腾讯云镜像(国内首选)/ Google 官方自动下载 platform-tools,缓存复用;支持 `ANDROIDFS_ADB_MIRROR` 自定义源

## 截图

(待补充)

## 技术栈

| 层 | 技术 |
|----|------|
| 桌面框架 | [Wails v3](https://wails.io) (`v3.0.0-alpha.95`) |
| 后端 | Go 1.25 |
| 前端 | React 18 + TypeScript + Vite |
| 设备通信 | ADB(内置二进制,运行时自动下载) |
| 构建 | Taskfile |
| CI | GitHub Actions |

## 架构

```
┌──────────────────────────────────────────────┐
│  前端 (React + TS)                            │
│  双栏 UI · hooks · 经 bindings 调后端 · 事件订阅 │
└──────────────────────┬───────────────────────┘
                       │ Wails bindings + Events
┌──────────────────────┴───────────────────────┐
│  后端 (Go, Wails 服务)                         │
│  app_*.go  ——  设备/浏览/传输/文件操作 服务方法  │
│  internal/                                     │
│    adb/        AdbClient: 调 adb 二进制 + 解析   │
│    localfs/    本地目录读取                      │
│    queue/      并发受限传输队列                   │
│    transfer/   传输引擎                          │
│    model/      数据结构                          │
└──────────────────────┬───────────────────────┘
                       │
                内置/自动下载的 adb 二进制
```

**核心包职责:**
- `internal/adb` —— 封装 adb 调用(设备列表、连接、shell、列目录、Stat、Mkdir/Rename/Delete、Push/Pull 带进度),解析 `ls -la` 与 stderr 进度输出
- `internal/localfs` —— 读取宿主机目录,返回与设备侧相同的 `FileEntry` 结构
- `internal/queue` —— 并发受限的任务队列(可配置并发数)
- `internal/transfer` —— 执行 adb push/pull 的引擎,带进度回调

## 快速开始

### 环境要求

- Go ≥ 1.25
- Node.js ≥ 18
- [Taskfile](https://taskfile.dev):`go install github.com/go-task/task/v3/cmd/task@latest`
- Wails v3 CLI:`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- macOS:Xcode Command Line Tools;Linux:`libgtk-4-dev libwebkitgtk-6.0-dev libsoup-3.0-dev`

### 开发运行

```bash
task dev          # 热重载开发模式
```

### 构建

```bash
task build                  # 当前平台二进制
task build:darwin:arm64     # 指定平台交叉编译
task package:darwin         # macOS .app 包(含图标、签名)
```

完整任务见 `Taskfile.yml`。

## 使用

1. Android 设备:**设置 → 开发者选项** 开启 **USB 调试**,USB 连接电脑
2. 打开 AndroidFS,设备会自动出现在顶部下拉框(若显示 `unauthorized`,在设备上点允许 USB 调试授权)
3. 左栏浏览本地文件,右栏浏览设备文件,选中后点「推送/拉取」或直接拖拽互传
4. 无线连接:点「无线连接」,输入 `ip:port`(设备需先执行 `adb tcpip 5555`)

> **关于全盘访问:** 设备面板默认从 `/` 起。非 root 设备对根目录多数条目无权限,会显示错误;root 设备可访问整个文件系统。普通使用建议进入 `/sdcard`、`/storage` 等可读目录。

## 测试

```bash
task test                  # 离线单元测试(-race)
go test -tags=integration ./internal/adb/... -run TestEnsureDownloadedLive -v
                           # 集成测试:真连镜像验证 adb 自动下载
```

## adb 自动下载说明

首次启动时,AndroidFS 检测缓存目录无 adb,按以下顺序下载 platform-tools:

1. `ANDROIDFS_ADB_MIRROR` 环境变量指定的自定义源(若有)
2. 腾讯云镜像 `mirrors.cloud.tencent.com/AndroidSDK/`(国内首选)
3. Google 官方 `dl.google.com/android/repository/`(兜底)

下载后解压到缓存目录并复用,之后启动零网络。任一源失败会自动尝试下一个;全部失败则回退到系统 PATH 的 adb(若已安装)。

## 设计

界面采用 **Precision Instrument(精密仪表盘)** 风格:深色底(`#0E1116`),单一薄荷青强调色(`#38E0C8`)——**仅在数据流动时点亮**;IBM Plex Sans(UI)+ IBM Plex Mono(路径、权限、文件大小、传输遥测)。中缝的传输遥测是标志性元素,实时显示速率与进度。配色/字体全部走 CSS 变量(`frontend/src/styles/tokens.css`),组件零行内 hex。

## 项目结构

```
├── .github/workflows/      # CI: build.yml(测试) + release.yml(发版)
├── app_*.go                # Wails 服务方法(设备/浏览/传输/文件操作)
├── main.go                 # 应用入口
├── internal/
│   ├── adb/                # ADB 客户端 + 解析器 + 自动下载
│   ├── localfs/            # 本地目录读取
│   ├── model/              # Device / FileEntry / TransferTask
│   ├── queue/              # 传输任务队列
│   └── transfer/           # 传输引擎
├── frontend/
│   ├── src/{hooks,components,styles}   # 前端源码
│   └── bindings/           # Wails 自动生成的 TS 绑定
├── docs/superpowers/       # 设计 spec 与实现计划
├── Taskfile.yml            # 构建任务
└── wails.json
```

## License

(待定)
