# [FT-DEV-001] [Tech Story] 构建/开发环境 Makefile 守护生命周期设计

> 本文档基于 `sounds-great-ai` **当前代码实况**（`Makefile`、`scripts/daemon-helper.sh`、`cmd/server/handlers.go`）逐文件梳理，描述 Makefile 目标体系与守护进程（daemon）生命周期的**设计意图、结构契约与安全边界**。
> 不涉及任何历史事件，仅解释"这套构建/运行工具链为什么这样设计、各 target 如何协作、有哪些约定与护栏"。
> 所有描述以当前代码状态为准。

---

## 1. 元信息与设计价值 (Context & Value)

- **类型**: [x] Tech Story (架构/重构/技术债)
- **责任人**: PO: @operator | Dev: @bianmu | QA: @demu
- **故事点/复杂度**: [ M (3-5分) ]
- **设计目标**:
  - As a **本地开发者 / operator**,
  - I want to **`make dev daemon` / `make prod daemon` 的启动与回收行为可预期、可观测、跨平台，且二进制名与项目强绑定**,
  - So that **改完代码重启后运行态与源码态始终一致，不会误伤其他项目的进程，排障时一眼能看出端口被谁占用**。
- **关键指标/埋点**: 无（本地构建/运行工具链，非对外服务）。

### 设计原则（贯穿全文）

1. **可预期**：任何 `make xxx daemon` 要么干净启动、要么明确拒绝，绝不"看似成功实则失败"。
2. **可观测**：端口被占用时报告占用者 PID 与命令，而非静默 bind 失败。
3. **安全**：只回收"自己人"进程，外来进程一律只报告、不动作。
4. **跨平台**：mac / linux / windows 三套环境下行为一致。
5. **命名强绑定**：二进制名带项目前缀 `sounds-great-ai`，避免与通用名 `server` 混淆。

---

## 2. 命令契约 (Command Schema)

所有对外命令的表面语义，是理解整个设计的第一入口：

```
make              → 打印帮助（.DEFAULT_GOAL := help）
make dev          → 前台启动后端(:8080) + 前端(:5173)，Ctrl+C 收尾，终端不能关
make dev daemon   → 后台启动，日志 .logs/，pid .pids/；端口预检 + 原子 pidfile
make prod         → 前台启动生产构建后端（前端已 build，由后端 serve）
make prod daemon  → 后台启动生产构建后端（不启动前端进程）
make stop         → 回收受管进程(.pids) + 端口兜底回收自家孤儿(is_ours)
make restart      → stop + dev daemon（一键干净重启）
make clean        → 删 web/dist、bin/、残留二进制(sounds-great-ai*)
make deep         → stop + clean + 删 .logs/.pids + 删 SQLite + go cache + node_modules + 删 .sounds-great-ai(运行时配置/凭据)
make upgrade      → 拉代码(prompt) + install + build 前端 + go build 后端（不 restart）
```

---

## 3. Makefile target 体系设计

### 3.1 顶层调度：`daemon` 是"开关"，不是目标

`Makefile` 第 7–12 行：

```makefile
ifneq ($(filter daemon,$(MAKECMDGOALS)),)
DAEMON_MODE=true
endif
daemon:
	@:
```

`daemon` 体内容是 `@:`（空操作），它**存在的唯一目的**是：命令行里出现 `daemon` 这个词时，`filter` 把 `DAEMON_MODE` 置为 `true`。于是 `make dev daemon` 实际执行 `dev`，但 `dev` 内部 `if [ "$(DAEMON_MODE)" = "true" ]` 分支被激活 → 进入后台模式。

**设计意图**：用一个无副作用的"模式开关"复用同一份 `dev`/`prod` 骨架，避免把前台/后台逻辑拆成 4 个 target。`.PHONY` 声明所有伪目标（防止与同名文件冲突），`.DEFAULT_GOAL := help`（裸 `make` 打印帮助）。

### 3.2 双入口骨架（dev / prod 同一副骨架）

`dev` 与 `prod` 共享结构，差异仅在"前台 / 后台"与"是否带前端"：

- **前台分支**（不带 daemon）：`trap 'kill 0' EXIT INT TERM` 注册清理钩子，将后端（`go run`）/前端（`npm run dev`）作为后台子进程拉起，父进程 `wait`。Ctrl+C 触发 `kill 0` 杀掉整个进程组，但要求终端保持打开。
- **后台分支**（带 daemon）：走 3.3 的三道关，输出 pidfile 后父进程退出，子进程独立存活。

**关键差异**：`prod` 不启动前端进程——生产模式下前端已被 `build` 成静态文件，由 Go 后端直接 serve，故 prod daemon 只管后端。这一差异直接决定了 3.4 与第 6 节中关于"前端残留"的设计权衡。

### 3.3 守护生命周期三道关（后台 daemon 分支）

后台启动不再是"直接 build + 起进程 + 写 pidfile"，而是依次过三关，确保"可预期"原则落地：

1. **守卫（第 22–31 行）**：遍历 `.pids/backend.pid`、`.pids/frontend.pid`。
   - 进程存活 → 打印 `Error: <name> already running (PID X). Run 'make stop' first.` 并 `exit 1`（**明确拒绝**，不静默、不重复拉起）。
   - 进程已死 → 先 `rm` 掉 stale pidfile 再继续。
2. **端口预检（第 36–37 行）**：`ensure_port 8080 1`（后端，`reclaim=1` 可回收自家孤儿）+ `ensure_port 5173 0`（前端，`reclaim=0` 不自动杀）。调 `scripts/daemon-helper.sh`，详见第 4 节。
3. **原子写 pidfile（第 44–46 行）**：进程起来后 `sleep 1`，`kill -0` 确认存活才写 pidfile；若已死则 `tail -15 backend.log` 输出失败原因并 `exit 1`。**这是"启动即失败绝不残留 stale pidfile"的保证**。

### 3.4 `stop` 的双层回收与安全边界

`stop`（第 116–149 行）做两件事，顺序固定：

- **第一层 · pidfile 回收**：按 `.pids/*.pid` 找到受管进程，`kill` → 等待 → 仍存活则 `kill -9`，然后删 pidfile。
- **第二层 · 端口兜底**：扫 `8080 / 9464 / 5173`，`port_pid` 拿 PID，`is_ours` 判定是自家进程才 `kill_pid` 回收。

**安全边界的核心**在 `is_ours`：只认命令含 `sounds-great-ai` 的进程才算"自家"，外来进程（其他项目占端口的服务）**永远只报告 PID、绝不动作**。这把"误杀"从设计上排除掉了。

### 3.5 `clean` / `deep` / `upgrade` / `restart` 的设计意图

| target | 设计意图 |
|--------|---------|
| `clean` | 删产物（`web/dist`、`bin/`、根目录残留二进制）。**不删 `.pids`/`.logs`、不回收进程**——它是纯"清文件"语义，进程回收交给 `stop` |
| `deep` | `deep: stop clean` + 删 `.logs/.pids`/SQLite/go cache/node_modules + 删 `.sounds-great-ai`。**把"先停后清"钉死在依赖顺序里**，防止"删了文件却留下孤儿进程"；删除 `.sounds-great-ai` 使 `deep` 成为"从零重置"语义，但会**不可逆清除运行时凭据** |
| `upgrade` | 拉代码 + install + build 前端 + `go build` 后端，但**刻意不 restart**——升级只更新磁盘二进制，是否重启由 operator 决定（保留人工确认点） |
| `restart` | `restart: stop` 后 `$(MAKE) dev daemon`，提供"一键干净重启"的便捷入口 |

---

## 4. 端口感知与跨平台设计 (`scripts/daemon-helper.sh`)

这是整条生命周期的"单一事实源"，把"端口 → 进程 → 是否自家 → 是否回收"的判定从 Makefile 抽离成可单测的脚本。按 `os_detect`（`uname` 区分 mac / linux / windows）选择底层命令：

| 函数 | 职责 | 跨平台实现 |
|------|------|-----------|
| `os_detect` | 返回 mac / linux / windows（MINGW/MSYS/CYGWIN 归 windows） | `uname -s` |
| `port_pid <port>` | 拿监听该 TCP 端口的 PID | mac/linux：`lsof`；linux 无 lsof 回退 `ss -ltnp`；windows：`netstat -ano` |
| `proc_cmd <pid>` | 拿 PID 的命令行/可执行路径 | mac/linux：`ps -o command=`，失败回退 `lsof -p` 的 `txt` 行；windows：`tasklist` |
| `is_ours <pid>` | **只认命令含 `sounds-great-ai`**（含 `-dev` 与 Windows `.exe`）→ 0，否则 1 | 基于 `proc_cmd` 字符串匹配 |
| `kill_pid <pid>` | SIGTERM → 等待 → 仍存活 SIGKILL | 通用 |
| `ensure_port <port> <reclaim>` | 空闲→OK；自家且 `reclaim=1`→回收；自家且 `reclaim=0`→拒绝；外来/legacy→拒绝并给精准提示 | 组合上述函数 |

### `ensure_port` 语义契约

| 端口 | reclaim | 占用者 | 行为 |
|------|---------|--------|------|
| 8080 | 1 | 空闲 | OK，继续 |
| 8080 | 1 | 自家 `sounds-great-ai` | 自动回收，返回 0 |
| 8080 | 1 | 外来 / legacy 通用名 | 拒绝，exit 1，给出 PID + 手动清理提示 |
| 5173 | 0 | 任意非自家 | 不自动杀；一律拒绝 exit 1（前端进程不在 `is_ours` 白名单内，见第 6 节） |

**设计取舍**：`is_ours` **故意不匹配通用名 `server`**。这是命名约定的直接延伸——正因为二进制改名带项目前缀，`is_ours` 才能用前缀精确识别"自己人"，既不会漏收自家孤儿，也不会误杀其他服务的同名进程。

---

## 5. 二进制命名设计

二进制名统一为 `sounds-great-ai`（dev 变体 `sounds-great-ai-dev`），与生产环境的 `sounds-great-mcp-server`（MCP 子服务）命名风格一致。涉及三处必须保持一致：

| 位置 | 作用 | 命名 |
|------|------|------|
| `Makefile` dev/prod/upgrade 构建输出 | `make` 启动的二进制 | `bin/sounds-great-ai[-dev]`（Windows `.exe`） |
| `Makefile` `clean` 删除列表 | 清理旧产物 | 含 `sounds-great-ai*`，同时清掉历史遗留的 `server`/`server-dev` |
| `cmd/server/handlers.go` 应用内自升级 | 自升级落盘的二进制名 | `bin/sounds-great-ai`（Windows `.exe`），source 模式 build 与 release 模式下载均同步 |

**命名一致性的必要性**：应用内自升级（`handlers.go`）会写出二进制，若它与 `make` 启动的目标名不一致，就会出现"自升级更新了 A 文件、实际跑的却是 B 文件"的错位。三处统一命名从根上消除这类错位。

---

## 6. 设计权衡与待完善项

以下为当前设计**有意识的取舍或已知限制**，非缺陷，列此供后续迭代评估：

1. **前端 vite 孤儿不在 `is_ours` 白名单内**：vite 命令是 `node .../vite`，不被认作"自家"。若 `5173` 被遗留 vite 占住，`ensure_port 5173 0`（reclaim=0 且非自家）会拒绝，且 `make stop` 端口兜底也清不掉它。当前设计为"前端残留交由人工处理"，避免把回收范围扩大到 node 进程族（风险更大）。
2. **`restart` 写死成 `dev daemon`**：`restart: stop` 后固定 `$(MAKE) dev daemon`，重启 prod 需手动 `make stop && make prod daemon`。保留 dev 为默认是出于"本地开发最常用"的假设。
3. **prod daemon 守卫仍检查 `frontend.pid`**：prod 不写 `frontend.pid`，但守卫循环统一查两个 pidfile。若之前 dev daemon 留下 `frontend.pid`，再 `make prod daemon` 会被"frontend already running"拦下——prod 守卫理论上只需查 backend。
4. **`upgrade` 与 `prod` 的 build 步骤重叠**：`upgrade` 内 `$(MAKE) build` + `go build` 与 `prod` 重复，属冗余不致命。
5. **`deep` 会不可逆清除运行时凭据**：`deep` 现删除仓库根 `.sounds-great-ai` 目录（含 `credentials.json` 0600 密钥、`accounts.json`、`dog-catalog.json`）。该目录已在 `.gitignore` 内、属 gitignored 的运行时数据，**不进版本库**。删除后下次启动按 `packs/default/breeds/dog-template.json` 重新生成种子。这是"从零重置"语义的有意延伸，但意味着 `deep` 不再只是"清构建产物"——operator 若只想清构建产物、保留账号/密钥/成员配置，应改用 `make clean`（浅清理，不含 `stop` 与目录删除）。此行为已同步到 `make help` 文案。

---

## 7. 验收标准 (Acceptance Criteria - AC)

- [x] **AC-01 (正常路径)**: Given 端口空闲，When `make dev daemon`，Then 构建 `bin/sounds-great-ai-dev` 并后台启动，`.pids/backend.pid` 写入存活 PID，`:8080` 与 `:5173` 均进入监听。
- [x] **AC-02 (异常与边界)**:
  - When 端口 8080 被自家 `sounds-great-ai` 孤儿占用，Then `make dev daemon` 自动回收该进程后启动（reclaim=1）。
  - When 端口 8080 被遗留通用名或外来进程占用，Then `make dev daemon` **拒绝**并报告占用者 PID，不静默失败。
  - When 上一次 `make dev daemon` 仍在跑（pidfile 存活），Then 重复执行 → `Error: ... already running ... Run 'make stop' first.` 明确拒绝。
  - When 后端启动即崩溃，Then 不写 stale pidfile，打印 `backend.log` 末尾并 `exit 1`。
  - When `make stop`，Then 同时回收 `.pids` 受管进程与无 pidfile 的端口孤儿（仅限 `is_ours` 命中）。
- [x] **AC-03 (权限与安全)**:
  - `is_ours` 只认 `sounds-great-ai`，When 端口被其他项目进程占用，Then `make stop` / `ensure_port` **绝不误杀**，仅报告 PID。
  - 跨平台：mac（lsof）、linux（lsof 或 ss 回退）、windows（netstat）均按 `os_detect` 选择正确命令。
  - 二进制改名后，`make` 启动名、自升级输出名、clean 删除名三者一致，无错位。
- [x] **AC-04 (`deep` 运行时数据清理范围)**:
  - When 执行 `make clean deep`，Then 除既有构建产物/日志/pid/SQLite/go cache/`node_modules` 外，还删除仓库根 `.sounds-great-ai`（accounts/credentials/dog-catalog 落盘）。
  - When 仅执行 `make clean`（浅清理），Then **不**删除 `.sounds-great-ai`、不回收进程——保留运行时账号/密钥/成员配置。
  - `.sounds-great-ai` 已在 `.gitignore`，删除不触动版本库；下次启动按 `dog-template.json` 重新种子化。

---

## 8. 稳定性与工程护栏 (Engineering & Stability Guardrails)

- **[x] 资损与网络安全 (Security)**: `clean`/`stop` 无敏感数据——核心护栏是"不误杀外来进程"，`is_ours` 白名单严格限定 `sounds-great-ai`，不会误伤其他服务。**但 `deep` 例外**：它现在会删除仓库根 `.sounds-great-ai`（含 `credentials.json` 0600 密钥、账号、成员目录），这是**不可逆的运行时凭据清除**。该目录 gitignored、不进版本库，删除以"从零重置"为预期。operator 须意识到 `deep` 已超出"清构建产物"范畴；仅清产物请用 `clean`。`deep` 在删除 `.sounds-great-ai` 前会先 `stop`（依赖 `deep: stop clean`），确保不删仍在运行的进程所对应的落盘数据时的进程状态已停止。
- **[x] 高并发与限流降级**: 不涉及（本地单机开发工具链）。
- **[x] 可服务性与监控**: `ensure_port` / `stop` 输出可读的占用者 PID 与命令，便于人工干预；`make dev daemon` 启动失败时输出 `backend.log` 末尾，缩短排障路径。

---

## 9. Story 级 Definition of Done (DoD Checklist)

- [x] 设计三要素锁定：端口感知生命周期 + 跨平台 + 二进制改名。
- [x] 构建门禁：`go build ./cmd/server/` 0、`make -n dev daemon` 语法 OK。
- [x] 行为验证：legacy 占用拒绝、pidfile 守卫拒绝、正常启动监听均已实跑确认。
- [x] 命名一致性：Makefile 启动名、clean 删除名、自升级输出名三处统一为 `sounds-great-ai`。
- [ ] 跨平台实跑：mac 已验证；linux（ss 回退路径）、windows（netstat 路径）需在实际环境各跑一次 `make dev daemon` + `make stop` 确认。
- [ ] 第 6 节 4 项待完善项可后续立项（尤其前端 vite 孤儿盲区）。

---

## 10. 修订记录 (Revision History)

- **2026-08-12（初版）**：以设计视角梳理 `Makefile` 目标体系与守护生命周期——`daemon` 开关、双入口骨架、启动三道关、`stop` 双层回收、`clean/deep/upgrade/restart` 意图、跨平台 `daemon-helper.sh`、`sounds-great-ai` 命名约定与安全边界。落盘为 Tech Story（纯设计文档，不含事件背景）。
- **2026-08-13（拆分恢复为独立文档）**：此前被并入 `FT-MEM-001-member-management.md` 的附录 A；经用户确认 Makefile/守护进程生命周期应独立于「成员管理」成文，故从合并文档中剔除并恢复为本独立 Tech Story（内容未改）。
- **2026-08-13（`deep` 扩展删除 `.sounds-great-ai`）**：应需求将 `make clean deep` 的清理范围扩展到项目根 `.sounds-great-ai` 运行时目录。`Makefile` 的 `deep` target 新增 `rm -rf .sounds-great-ai`（位于 `node_modules` 清理之后、`go clean` 之前），`make help` 文案同步标注。随之更新本文：§2 命令契约、§3.5 `deep` 行、§6 增补"不可逆清除运行时凭据"取舍项、§8 资安护栏改写（区分 `clean`/`stop` 无敏感数据 vs `deep` 删凭据）。

---

> 改动文件：`Makefile`、`scripts/daemon-helper.sh`、`cmd/server/handlers.go`。
> 环境备注：本沙箱 `ps`/`lsof -p` 受限且后台子进程随 shell 退出被杀，故"跨命令持久化"与"按端口自动回收"未在此沙箱实跑，逻辑已审查、真实 macOS 终端满足这两个条件。
