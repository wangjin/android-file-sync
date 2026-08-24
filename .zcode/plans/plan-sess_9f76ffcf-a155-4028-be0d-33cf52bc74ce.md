# 合并 release.yml 的 windows/macos 构建为 matrix job

## 目标
消除 `build-windows` 与 `build-macos` 两个 job 里约 20 行重复的 checkout / setup-go / setup-node / cache / install 步骤，合并成一个基于 matrix 的 `build` job。产物、并行性、发版流程不变。

## 改动文件
仅 `.github/workflows/release.yml`。

## 具体改动

### 1. 用 matrix 替换两个 job
把 `build-windows`(L31-54) 和 `build-macos`(L56-85) 整体替换为单个 `build` job：

```yaml
build:
  needs: test
  strategy:
    fail-fast: false          # 一个平台失败不取消另一个，便于定位
    matrix:
      include:
        - os: windows-latest
          package_task: package:windows
          artifact_name: AndroidFS-windows-amd64
          artifact_path: build/bin/AndroidFS-windows-amd64.exe
        - os: macos-26
          package_task: package:darwin
          artifact_name: AndroidFS-macos
          artifact_path: build/bin/AndroidFS-macos.dmg
  runs-on: ${{ matrix.os }}
  steps:
    - uses: actions/checkout@v5
    - uses: actions/setup-go@v6
      with:
        go-version: "1.26"
    - uses: actions/setup-node@v5
      with:
        node-version: "22"
        cache: npm
        cache-dependency-path: frontend/package-lock.json
    - uses: actions/cache@v5
      with:
        path: ~/go/bin
        key: go-tools-androidfs-${{ runner.os }}-${{ hashFiles('go.sum') }}
    - run: go install github.com/wailsapp/wails/v3/cmd/wails3@latest
    - run: go install github.com/go-task/task/v3/cmd/task@latest
    - run: task ${{ matrix.package_task }}
    # macOS 独有：把 .app 打成 .dmg（windows 直接用 .exe）
    - name: Create macOS .dmg
      if: matrix.os == 'macos-26'
      run: |
        mkdir -p /tmp/dmg-content
        cp -R build/bin/AndroidFS.app /tmp/dmg-content/
        ln -s /Applications /tmp/dmg-content/Applications
        hdiutil create -volname "AndroidFS" -srcfolder /tmp/dmg-content -ov -format UDZO build/bin/AndroidFS-macos.dmg
    - uses: actions/upload-artifact@v5
      with:
        name: ${{ matrix.artifact_name }}
        path: ${{ matrix.artifact_path }}
```

### 2. 更新 release job 的 needs
- `release.yml:88` `needs: [build-windows, build-macos]` → `needs: [build]`
- 其余（download-artifact、release notes、上传 .exe/.dmg）完全不变 —— artifact 名字和路径与现状一致。

## 不变的东西（验证过）
- **并行性**：matrix 的两个条目默认并行执行，和原来两个 job 一样快。
- **artifact 命名**：`AndroidFS-windows-amd64` / `AndroidFS-macos` 与原来一致 → `release` job 里 `artifacts/AndroidFS-windows-amd64/*.exe`、`artifacts/AndroidFS-macos/*.dmg` 路径不变。
- **cache key**：仍用 `${{ runner.os }}`，windows/macos 各自独立缓存，不串。
- **test job**（ubuntu，跑 `task test`）保持独立，作为 `build` 的 `needs` 前置。
- **build.yml**（PR 验证）不改。
- **Taskfile.yml** 不改。

## 新增的小改进
- `fail-fast: false`：原配置没显式设（默认 true）。加上后，比如 windows 挂了不会把 macos 也一起取消，方便排查。这是 matrix 化后唯一的行为差异，属于改进。

## 验证方式
1. `yq` 或 `actionlint` 校验 YAML 语法与 workflow 结构。
2. 无法本地跑完整 CI；最终以打 tag 触发后的实际运行为准。改动是纯结构重排，逻辑等价。

## 风险
- 极低。差异点（package task、artifact 名/路径、mac dmg 步骤）全部由 matrix 变量 + 一个 `if` 条件精确表达，无逻辑变更。