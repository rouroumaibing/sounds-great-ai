# [FT-UPG-001] [Tech Story] 版本升级与前端发布链路设计

> 本文档基于 `sounds-great-ai` **当前代码实况**（`Makefile`、`cmd/server/handlers.go`、`cmd/server/routes.go`、`web/embed_*.go`、`web/vite.config.ts`、`web/src/sw.ts`、`web/src/services/update.ts`、`web/src/components/common/{ErrorBoundary,UpdateBanner}.tsx`、`web/src/main.tsx`）逐文件梳理，描述"一个已部署实例如何升级、一个已打开的浏览器页面如何跨升级存活"的**前后端完整链路**。
> 不涉及历史事件，仅解释这套机制为什么这样设计、各环节如何协作、有哪些硬约束。
> 所有描述以当前代码状态为准。

---

## 1. 元信息与设计价值 (Context & Value)

- **类型**: [x] Tech Story (架构/重构/技术债)
- **责任人**: PO: @operator | Dev: @bianmu | QA: @demu
- **故事点/复杂度**: [ L (8-13分) ]
- **设计目标**:
  - As a **本地开发者 / operator**,
  - I want to **升级（`make prod daemon` / `make upgrade` / 应用内升级）前后，一个已经打开的浏览器页面要么无感换血、要么自动恢复，且升级过程任何时刻线上资源都是完整可服务的**,
  - So that **升级不再产生"点了路由报 MIME 错误/404、必须手动强刷"的破坏窗口，前后端版本永远配套**。
- **关键指标/埋点**: 无本地埋点；验收依赖 AC 节的 curl 行为断言。

### 设计原则（贯穿全文）

1. **存活客户端优先**：升级的验收视角是"开着的 tab 会经历什么"，不是"构建是否成功"。
2. **诚实失败**：缺失的静态资源返回真 404（而非 index.html 冒充），缓存语义用显式 `Cache-Control` 表达，不做任何静默兜底。
3. **单一产物原子**：磁盘 dist 目录取一次 rename 完成交换；release 形态把前端嵌进二进制，升级一个文件即前后端同步换血。
4. **契约可验证**：集合字段永不为 `null`、版本号来自真实 git 状态、构建产物含关键内容——均有测试或验收命令锁定。

---

## 2. 升级路径总览

三条升级路径最终都收敛到同一组机制（原子交换 + build-id 三源解析 + 版本注入 + 前端自愈）：

```
路径 A: make prod daemon ──→ make build（dist.new → 原子交换，旧构建 → dist.old）
                              → go build -tags embeddist -ldflags "版本+build-id"
                              → stop 旧进程 → 启动新二进制
路径 B: make upgrade ───────→ git pull(prompt) → make install → make build →
                              go build（同 A 的标签与 ldflags）→
                              若 .pids/backend.pid 存活 → make stop + make prod daemon 自动重启
路径 C: 应用内 POST /api/upgrade
        source 模式 ───────→ server 侧 exec: git pull(可选) → make install → make build →
                              go build -tags embeddist -ldflags（goBuildLdflags()）
                              → 响应提示需重启（不自杀）
        release 模式 ──────→ 从 GitHub release 下载平台二进制替换 bin/sounds-great-ai
                              → 前端由二进制内嵌副本提供（build-id 比较自动胜出）
```

**路径 C release 模式的成立前提**：二进制以 `-tags embeddist` 构建，内嵌了配套前端（第 3.2 节）。当前 `.github/workflows/` 仅有 ci.yml、无 release 产物流水线，release 模式暂无实际产物来源——这是已知限制（第 7 节），机制本身已就位。

---

## 3. 后端：静态资源服务与版本解析

### 3.1 SPAHandler 请求语义（`cmd/server/handlers.go`）

`routes.go` 末尾 `mux.Handle("/", SPAHandler(workspaceDir))` **无条件注册**（不再以 dist 存在为前提；三个源都不可用时自然 404）。每个请求的判定顺序：

1. `/api/` 前缀 → 直接 404（API 未匹配路径不进 SPA）。
2. 路径经 `path.Clean("/"+path)` 规整（天然消灭 `..` 穿越），空路径映射为 `index.html`。
3. **点文件（`.build-id` 等）永不服务**：非导航请求 404，导航请求改写为 `index.html` 走兜底。
4. 导航判定：`Sec-Fetch-Dest: document` **或** `Accept` 含 `text/html`。
5. 依次查 **primary → secondary → grace** 三棵资源树（3.2 节），命中即返回。
6. 全部未命中时：**导航请求** fallback 到 primary 的 `index.html`；**非导航请求返回真 404**。这是与旧行为的本质区别——旧实现把缺失的 `/assets/*.js` 也回退成 index.html，浏览器对 module script 严格 MIME 校验直接拒绝，这就是"升级后旧 tab 点路由报 text/html MIME 错误"的根源。

### 3.2 三棵资源树与 build-id 排名（`spaFS` / `rankSpaRoots`）

| 树 | 来源 | 可用条件 | build-id |
|---|---|---|---|
| disk | 磁盘 `web/dist` | 目录含 `index.html` | `.build-id` 文件（unix 秒），缺失回退 `index.html` mtime |
| embedded | 二进制内嵌副本（`web/embed_dist.go` 的 `go:embed all:dist`） | 内嵌树含 `index.html` | ldflags 注入的 `main.embeddedBuildID` |
| grace | 磁盘 `web/dist.old`（上一代构建） | 同 disk | 同 disk 规则 |

排名规则（`rankSpaRoots`）：embedded 可用且（disk 不可用 **或** `embedded.id > disk.id`）→ embedded 为 primary；否则 disk 为 primary，另一方为 secondary。grace 永远排在查找链末位。

**这个排名解决两个真实场景**：
- **release 升级**（路径 C release）：磁盘 dist 是旧版、二进制内嵌新版 → embedded.id 更大 → 服务内嵌副本，前后端不漂移。
- **本地只重构建前端**（vite build 不重编译 Go）：磁盘 dist 的 id 更大 → 立即生效，无需重编译。

**grace 的用途**：`make build` 交换后，旧构建在 `dist.old` 存活一代。升级前已打开的 tab 仍引用旧哈希 chunk（如 `MemberManagement-puA6Ngbc.js`），请求会落到 grace 命中返回——给在途页面一个构建代的宽限窗口。连续两次构建后最老 chunk 消失，那时旧 tab 早已通过第 5 节的自愈机制换血。

### 3.3 缓存头契约（`serveSpaFile`）

| 路径 | Cache-Control | 理由 |
|---|---|---|
| `assets/*` | `public, max-age=31536000, immutable` | 文件名含内容哈希，内容变名字变，可永久强缓存 |
| `index.html`、`sw.js`、`registerSW.js`、`manifest.json`、`*.webmanifest` | `no-cache` | 入口必须每次 revalidate，否则浏览器启发式缓存会让旧 index.html 引用已删除的哈希 |
| 其他（favicon.svg 等根级文件） | 无显式头 | 走 `http.ServeContent` 默认协商 |

文件本体用 `http.ServeContent` 输出：按扩展名给出正确 MIME（`.js → text/javascript`）、支持条件请求与断点。**MIME 正确性本身就是契约**——module script 的严格校验意味着任何把 JS 当 html 吐的行为都会变成前端故障。

### 3.4 版本号注入（`version` / `embeddedBuildID`）

两个包级变量由构建命令通过 `-ldflags -X` 注入：

- `main.version`：`git describe --tags --always --dirty`（无 tag 时为短 SHA + `-dirty` 后缀）。
- `main.embeddedBuildID`：构建时读 `web/dist/.build-id`（vite 构建产物里的 unix 秒时间戳）。

`GET /api/upgrade/info` 返回 `version`；未注入（裸 `go run`）时依次回退 `VERSION` 文件 → `"dev"`。前端以此探测"服务器已换新"（第 5.4 节）。

### 3.5 应用内升级（`UpgradeHandler`）

- **source 模式**：依次 exec `git pull`（请求体 `pull:true 时`）→ `make install` → `make build` → `go build -tags embeddist -ldflags $(goBuildLdflags()) -o bin/sounds-great-ai cmd/server/main.go`。`goBuildLdflags()` 在进程内重新计算版本与 build-id（与 Makefile 同一套来源），保证自升级产物与 make 构建语义一致。完成后响应提示重启，**服务器不会自杀**。
- **release 模式**：查 GitHub latest release 的平台资产（`GOOS-GOARCH` 匹配）下载替换 `bin/sounds-great-ai`。因产物已内嵌前端，替换后无需碰 `web/dist`——3.2 的排名会让内嵌新版自动生效。

---

## 4. 构建侧：Makefile 契约

### 4.1 `make build`：真实类型检查 + 原子交换

```
1. cd web && tsc --noEmit -p tsconfig.app.json && tsc --noEmit -p tsconfig.node.json
   （根 tsconfig 是 files:[]+references 的 solution 配置，裸 tsc --noEmit 是空操作）
2. vite build --outDir dist.new --emptyOutDir
3. rm -rf web/dist.old
4. [ -d web/dist ] && mv web/dist web/dist.old
5. mv web/dist.new web/dist
```

- vite 配置里的 `writeBuildId()` 插件在 `closeBundle` 时向产物写入 `.build-id`（unix 秒），这是 3.2 排名与 ldflags 注入的时间戳来源。
- 步骤 4-5 是**一次 rename 的原子交换**：运行中的服务器任何时刻看到的 `web/dist` 都是完整的一棵树，不存在"构建到一半"的可见状态。
- `dist.old` 只保留一代：每次 build 先删上一代 grace，再把刚被替代的构建挪进去。

### 4.2 `embeddist` 构建标签与 go:embed

| 文件 | 标签 | 内容 |
|---|---|---|
| `web/embed_dist.go` | `//go:build embeddist` | `//go:embed all:dist` → 真实嵌入 |
| `web/embed_stub.go` | `//go:build !embeddist` | 空 `embed.FS` 桩 |

**设计动机**：`go:embed` 的模式在编译期必须匹配到至少一个文件。若无条件嵌入，源码用户手动 `rm -rf web/dist` 后，裸 `go build ./...` / `go test ./...`（CI 与 QC 的 quality gate 都这么跑）会直接编译失败。加标签后：**默认构建永远可编译**（空桩，运行时只从磁盘 dist 服务）；`-tags embeddist` 只出现在一定先跑过 `make build` 的路径上（`make prod`、`make upgrade`、应用内 source 升级）。副作用是绕过 make 手动 `go build -tags embeddist` 且 dist 缺失时会得到清晰的 "no matching files" 报错——这是预期行为，不是缺陷。

### 4.3 版本注入与 `clean`/`upgrade`

```make
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GO_LDFLAGS = -X main.version=$(VERSION) -X main.embeddedBuildID=$$(cat web/dist/.build-id 2>/dev/null || echo 0)
```

`GO_LDFLAGS` 用递归展开（`=`），`$$(cat ...)` 在 recipe 执行时读取——同一命令里先 `make build` 再 `go build` 时读到的是刚写入的新 build-id。

- `make prod` / `make prod daemon`：`go build -ldflags "$(GO_LDFLAGS)" -tags embeddist`。
- `make dev daemon`：带 ldflags、**不带** embeddist（dev 前端走 vite:5173，嵌入无意义；不带标签也使 dev 在 dist 缺失时可编译）。
- `make upgrade`：构建完成后若 `.pids/backend.pid` 存活 → `make stop && make prod daemon` 自动重启（注意：这与 FT-DEV-001 旧版"刻意不 restart、重启由 operator 决定"的语义**已变更**，FT-DEV-001 已同步）。
- `make clean`：删 `web/dist`、`web/dist.new`、`web/dist.old`、`bin/` 与根目录残留二进制。三个 dist 目录全部清理，不残留 grace。

---

## 5. 前端：Service Worker 与页面自愈

### 5.1 SW 构建方式（`web/vite.config.ts`）

`VitePWA({ registerType: 'autoUpdate', strategies: 'injectManifest', srcDir: 'src', injectManifest: { globPatterns: [...] } })`。

**三个硬约束**（都是本链路曾实际踩中的坑，属于"改配置前必读"）：

1. **`workbox:` 键只在 `generateSW` 策略下生效**；`injectManifest` 策略读 `injectManifest:` 键，配错不报错、glob 静默回退默认值。
2. **`self.__WB_MANIFEST` 必须以字面量出现在 sw.ts 里**——vite-plugin-pwa 在构建产物中搜索该字面量做预缓存清单注入，经过变量别名（`const sw = self` 后写 `sw.__WB_MANIFEST`）打包后字面量消失，注入直接报 "Unable to find a place to inject the manifest"。
3. **`src/sw.ts` 是 SW 的唯一源码**。runtime 缓存、导航回退、push 处理器全部写在 sw.ts；generateSW 模式下这个文件会被静默忽略（整文件死代码）。

### 5.2 `web/src/sw.ts` 的运行时行为

- `precacheAndRoute(self.__WB_MANIFEST)` + `cleanupOutdatedCaches()`：全量构建产物（含全部懒加载 chunk 与字体）预缓存；新 SW 激活时清理旧代缓存。
- `NavigationRoute(createHandlerBoundToURL('index.html'), { denylist: [/^\/ws/] })`：SPA 导航回退，`/ws` 例外。
- `registerRoute(/\/api\//i, new NetworkOnly(), 'GET')`：API 永不缓存。
- 静态资源（png/jpg/svg/ico/woff/woff2）CacheFirst，`static-assets` 缓存，60 条 / 30 天过期。
- push 通知：`notif-dedup` Cache 去重（5 分钟窗口）、`notificationclick` 聚焦/新开 tab。
- 生命周期：install 即 `skipWaiting`、activate 即 `clients.claim()`、支持 `SKIP_WAITING` 消息——与 `registerType: 'autoUpdate'` 配合，新 SW 一经检出立即接管。

### 5.3 页面自愈三件套

| 位置 | 触发 | 动作 |
|---|---|---|
| `ErrorBoundary.componentDidCatch` | 错误特征匹配 `isStaleChunkError`（动态导入失败 / MIME text/html 拒绝） | `tryAutoReload()`（React lazy 缓存 rejected promise，就地重试无效，唯一恢复手段是整页刷新） |
| `main.tsx` 顶层 | `controllerchange`（新 SW 接管本页），仅限加载时已有 controller 的页面（排除首次注册） | `tryAutoReload()` |
| `UpdateBanner`（App 挂载） | 60s 轮询 + visibilitychange 时请求 `/api/upgrade/info`，版本与首测基线不同 | 显示"新版本已部署，立即刷新"横幅（**不自动刷新**——横幅是非破坏性提醒，自动换血交给 controllerchange 路径） |

`tryAutoReload()`（`web/src/services/update.ts`）：sessionStorage 键 `sg-auto-reload-at` 记录上次自动刷新时间，**30 秒节流**——三个触发源共享同一节流，防止"部署本身坏了"变成刷新死循环。

`UpdateBanner` 的每次轮询同时调用 `navigator.serviceWorker.getRegistration().then(r => r?.update())`：浏览器默认只在导航时检查 sw.js 更新，长驻不刷新的 tab 会一直用旧预缓存；主动触发把 SW 换代等待压缩到 ≤60s。

### 5.4 部署后浏览器时间线（各机制如何衔接）

```
T0  部署完成（dist 原子交换、二进制内嵌新版）
T0+ 打开着的旧 tab：入口 chunk 已在内存/SW 缓存，无需网络
T0+≤60s  UpdateBanner 轮询发现版本变化 → 横幅提醒
         同时 registration.update() → 新 SW 安装（skipWaiting+claim）
         → controllerchange → 页面自动 reload 换新
    （若用户在换血前点了懒加载路由：旧哈希请求 → dist.old 宽限命中，
      或宽限已过 → 真 404 → ErrorBoundary 识别 → tryAutoReload 自愈）
T1  新导航：index.html no-cache → 拿到新入口 → 引用新哈希 assets（immutable 强缓存）
```

---

## 6. 关联契约（本 story 落地时一并修复，详细条款见对应文档）

- **API 集合字段永不为 `null`**（Go nil slice 会序列化成 `null`，前端按 `string[]` 使用即崩）：`skills_handler.go toItem` 保证 `triggers`/`mountPoints` 空集输出 `[]`。契约条款见 `docs/reference/API.md`。
- **中间件包装必须保留 ResponseWriter 可选接口**（`Hijacker`/`Flusher`）：telemetry `statusRecorder` 补齐 `Flush` 委托，SSE 端点（`/api/people-memory/events`）不再 500。约定见 SOP 验收清单。
- **自指字符串（语言名/品牌名）不走 `t()`**：语言切换标签使用固定原生名（`中文`/`English`）。

---

## 7. 已知限制与权衡

1. **release 模式无产物来源**：`.github/workflows/` 只有 ci.yml。若启用 release 升级，需补 release 流水线，产物必须是 `-tags embeddist` 构建的单二进制（内嵌前端是路径 C 成立的前提）。
2. **`dist.old` 只有一代宽限**：连续两次部署之间没刷新的旧 tab，第二次部署后旧 chunk 彻底消失，只能靠 404 → ErrorBoundary 自愈。磁盘占用有上界（一代构建 ≈ 3MB）。
3. **`make upgrade` 会触发一次重复构建**：自动重启走 `make prod daemon`，其内部再次 `make build` + `go build`（FT-DEV-001 §6 已记录）。升级是低频操作，正确性优先于耗时。
4. **`make dev` 构建的二进制无内嵌前端**：dev 场景前端在 vite:5173，磁盘 dist 缺失时 `:8080` 无页面属预期。
5. **裸 `go build -tags embeddist` 在 dist 缺失时报错**：标签只应由 make 流程使用（见 4.2）。

---

## 8. 验收标准 (Acceptance Criteria - AC)

以下均已在真实部署上实跑：

- [x] **AC-01 (404 语义)**：`GET /assets/<不存在的哈希>.js`（非导航头）→ `404 + text/plain`，不再是 `200 + text/html`。
- [x] **AC-02 (缓存头)**：`assets/*` → `public, max-age=31536000, immutable`；`/`、`sw.js` → `no-cache`；JS 资源 MIME 为 `text/javascript`。
- [x] **AC-03 (原子交换)**：`make build` 后 `web/dist` 为新构建、`web/dist.old` 为上一代；仅存在于 `dist.old` 的旧 chunk 请求返回 200（宽限生效）。
- [x] **AC-04 (嵌入回退)**：移走 `web/dist` 后 `/` 与 assets 仍 200（二进制内嵌副本接管）；恢复后磁盘版生效。
- [x] **AC-05 (构建标签)**：删除整个 dist 后裸 `go build ./cmd/server/` 成功（空桩）；`go build -tags embeddist` 如实报 `no matching files`。
- [x] **AC-06 (版本注入)**：`/api/upgrade/info` 返回真实 git describe 值（如 `fdb524e-dirty`）。
- [x] **AC-07 (SW 产物断言)**：`dist/sw.js` 同时含 push 处理器（`notif-dedup`、`SKIP_WAITING`）与完整预缓存清单（150 项，含全部字体）。
- [x] **AC-08 (前端自愈)**：单测覆盖 `isStaleChunkError` 特征与 `tryAutoReload` 节流（`web/src/services/update.test.ts`）。
- [x] **AC-09 (升级自动重启)**：daemon 运行中执行 `make upgrade`，结束时自动 `stop` + `prod daemon`，新版本号生效。

---

## 9. Story 级 Definition of Done (DoD Checklist)

- [x] 后端：SPAHandler 三源解析 / 缓存头 / 404 语义 / 版本注入，配套 `cmd/server/spa_handler_test.go`。
- [x] 构建：原子交换、`embeddist` 标签、ldflags、`clean` 覆盖三个 dist 目录、`upgrade` 自动重启。
- [x] 前端：SW injectManifest 接入、ErrorBoundary/controllerchange/UpdateBanner 三路自愈、vite build-id 插件。
- [x] 文档：本 story + FT-DEV-001 同步 + API.md 契约条款 + SOP 验收清单（见第 6 节）。
- [ ] release 流水线立项（第 7.1 项），启用前路径 C release 模式视为未验证。

---

## 10. 修订记录 (Revision History)

- **2026-08-23（初版）**：以设计视角梳理版本升级全链路——SPAHandler 三源 build-id 解析与缓存/404 契约、Makefile 原子交换与 embeddist 标签、ldflags 版本注入、SW injectManifest 与前端三路自愈、三条升级路径时序。配套修复（数组非 null 契约、telemetry Flusher、i18n 自指字符串）以引用方式关联。

---

> 改动文件：`Makefile`、`cmd/server/handlers.go`、`cmd/server/routes.go`、`web/embed_dist.go`、`web/embed_stub.go`、`web/vite.config.ts`、`web/src/sw.ts`、`web/src/services/update.ts`、`web/src/components/common/{ErrorBoundary,UpdateBanner}.tsx`、`web/src/main.tsx`、`web/.gitignore`、`internal/telemetry/middleware.go`、`internal/transport/skills_handler.go`。
