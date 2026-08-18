#!/bin/bash
# scripts/pre-merge-check.sh — SG 合流前全量门禁
#
# merge-gate 的硬门禁脚本。在 squash merge 前，基于最新 origin/main
# 跑全量 Go build/test/vet + 前端 tsc/vite build，确保合流后仍然全绿。
# 内置四项硬化护栏（纯 bash 实现，零新增依赖）：
#   A. Worktree 位置守卫  —— 禁止在主仓库内部 worktree 跑 gate（防 web build 假红）
#   B. Gate 单飞锁        —— 并发 gate 互斥，过期锁自动接管
#   C. dirty-worktree ledger —— merge 前确认所有 worktree 的 dirty diff 有归属
#   D. gate-last-run sentinel —— 记录最近一次 gate 通过时间（供 freshness 判定）
#
# Usage:
#   ./scripts/pre-merge-check.sh             # 默认：rebase origin/main 后全量门禁（禁止在 main 上）
#   ./scripts/pre-merge-check.sh --no-rebase # 仅本地/CI 校验（不 fetch/rebase，可在 main 上运行）
#
#   注：main 分支上默认（带 rebase）会被拦截，以防改写 main 历史；加 --no-rebase
#   后仅在 main 上做全量校验，CI 的 pre-merge job 即走此路径。
#
# 退出码：全绿 0；任一步骤失败 1。

set -euo pipefail

NO_REBASE=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-rebase) NO_REBASE=true; shift ;;
    --help|-h)
      echo "Usage: scripts/pre-merge-check.sh [--no-rebase]"
      exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
WEB_DIR="$REPO_ROOT/web"

GATE_START=$SECONDS
record_step() { echo "  • $1: ${2}s"; }

# ── Step 0: 分支 + 脏工作区检查 ──
BRANCH="$(git branch --show-current 2>/dev/null)"
# 分支守卫：rebase 流程会把当前分支 rebase 到 origin/main，若在 main 上执行会改写
# main 的本地历史，原则上不允许。但 --no-rebase（CI 场景：detached HEAD 或已合流分支）
# 只做“验证当前树是否全绿”，在 main 上运行安全且有价值（等价于对 main 跑全量校验，
# 并补上 frontend CI job 未覆盖的 vite 生产构建检查），故仅在非 --no-rebase 时拦截。
if [ "$BRANCH" = "main" ] && [ "$NO_REBASE" = "false" ]; then
  echo -e "${RED}❌ 不能在 main 分支上执行带 rebase 的 gate 检查（会改写 main 本地历史）${NC}"
  echo -e "${RED}   如需对 main 做全量校验，请加 --no-rebase${NC}"
  exit 1
fi
UNCOMMITTED="$(git status --porcelain)"
if [ -n "$UNCOMMITTED" ]; then
  if [ "$NO_REBASE" = "true" ]; then
    echo -e "${YELLOW}⚠️ 检测到未提交改动，因 --no-rebase 继续本地验证${NC}"
  else
    echo -e "${RED}❌ 请先 commit 所有改动再执行 gate 检查${NC}"
    echo "$UNCOMMITTED" | head -10
    exit 1
  fi
fi
echo -e "${GREEN}✓ 分支: $BRANCH${NC}"

# ── 护栏 A: Worktree 位置守卫 ──
# 主仓库内部 worktree (.git/worktrees/ 等) 会让 Node 向上解析到兄弟目录的
# node_modules，造成 web build 假红。禁止在仓库内部创建 worktree。
MAIN_WORKTREE="$(git worktree list --porcelain | sed -n '1s/^worktree //p')"
if [ "$REPO_ROOT" != "$MAIN_WORKTREE" ]; then
  case "$REPO_ROOT" in
    "$MAIN_WORKTREE"/*)
      echo ""
      echo -e "${RED}❌ Worktree 在主仓库内部！${NC}"
      echo "   当前路径: $REPO_ROOT"
      echo "   主仓库:   $MAIN_WORKTREE"
      echo "   正确做法：git worktree add ../sounds-great-ai-{feature} -b feat/{name}"
      exit 1
      ;;
  esac
fi
echo -e "${GREEN}✓ Worktree 位置合规${NC}"

# ── 护栏 B: Gate 单飞锁（并发互斥）──
# 纯 bash：mkdir 原子抢锁；持锁 pid 存活则互斥退出；过期锁自动接管。
GATE_LOCK_DIR="${SG_GATE_LOCK_DIR:-$REPO_ROOT/.sounds-great-ai/gate/pre-merge-check.lock}"
mkdir -p "$(dirname "$GATE_LOCK_DIR")"
acquire_lock() {
  if mkdir "$GATE_LOCK_DIR" 2>/dev/null; then
    echo "$$" > "$GATE_LOCK_DIR/holder"
    return 0
  fi
  local holder
  holder="$(cat "$GATE_LOCK_DIR/holder" 2>/dev/null || echo "")"
  if [ -n "$holder" ] && kill -0 "$holder" 2>/dev/null; then
    echo -e "${RED}❌ 已有 gate 在运行 (pid $holder)，退出避免并发${NC}"
    exit 1
  fi
  rm -rf "$GATE_LOCK_DIR" 2>/dev/null || true
  if mkdir "$GATE_LOCK_DIR" 2>/dev/null; then
    echo "$$" > "$GATE_LOCK_DIR/holder"
    echo -e "${YELLOW}⚠ 发现过期 gate 锁，已接管${NC}"
    return 0
  fi
  echo -e "${RED}❌ 无法获取 gate 锁${NC}"
  exit 1
}
acquire_lock
release_lock() { rm -rf "$GATE_LOCK_DIR" 2>/dev/null || true; }
trap release_lock EXIT INT TERM
echo -e "${GREEN}✓ Gate 单飞锁已获取${NC}"

# ── Step 1: 同步 origin/main 并 rebase ──
if [ "$NO_REBASE" = "true" ]; then
  echo -e "${YELLOW}⚠ 跳过 origin/main rebase（--no-rebase）${NC}"
else
  echo "── Step 1/5: 同步 origin/main 并 rebase ──"
  for attempt in 1 2 3; do
    if git fetch origin main --quiet 2>&1; then break; fi
    if [ "$attempt" -eq 3 ]; then echo -e "${RED}❌ git fetch origin main 失败${NC}"; exit 1; fi
    echo -e "${YELLOW}⚠ fetch 重试 ($attempt/3)${NC}"; sleep 2
  done
  if ! git rebase origin/main --quiet 2>&1; then
    echo -e "${RED}❌ Rebase 有冲突，请手动解决后重新执行${NC}"
    exit 1
  fi
  echo -e "${GREEN}✓ rebase origin/main 成功${NC}"
fi

# ── Step 2: Go 全量 build ──
STEP_START=$SECONDS
echo "── Step 2/5: go build ./... ──"
if ! go build ./...; then
  echo -e "${RED}❌ go build 失败${NC}"; exit 1
fi
echo -e "${GREEN}✓ go build 通过${NC}"
record_step "build" $((SECONDS - STEP_START))

# ── Step 3: Go vet + test ──
STEP_START=$SECONDS
echo "── Step 3/5: go vet ./... ──"
if ! go vet ./...; then
  echo -e "${RED}❌ go vet 失败${NC}"; exit 1
fi
echo -e "${GREEN}✓ go vet 通过${NC}"
echo "── Step 3/5: go test ./... ──"
if ! go test ./...; then
  echo -e "${RED}❌ go test 失败${NC}"; exit 1
fi
echo -e "${GREEN}✓ go test 通过${NC}"
record_step "vet+test" $((SECONDS - STEP_START))

# ── Step 4: 前端 tsc + vite build ──
if [ -d "$WEB_DIR" ]; then
  STEP_START=$SECONDS
  echo "── Step 4/5: 前端 tsc -b + vite build ──"
  if ! (cd "$WEB_DIR" && exec node_modules/.bin/tsc -b); then
    echo -e "${RED}❌ 前端 tsc 失败${NC}"; exit 1
  fi
  if ! (cd "$WEB_DIR" && exec node_modules/.bin/vite build); then
    echo -e "${RED}❌ 前端 vite build 失败${NC}"; exit 1
  fi
  echo -e "${GREEN}✓ 前端 tsc + vite build 通过${NC}"
  record_step "web" $((SECONDS - STEP_START))
else
  echo -e "${YELLOW}⚠ 无 web 目录，跳过前端构建${NC}"
fi

# ── Step 5: 报告 ──
GATE_TOTAL=$((SECONDS - GATE_START))
FINAL_SHA="$(git rev-parse HEAD)"
SHORT_SHA="${FINAL_SHA:0:8}"
echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                  ✅ GATE PASSED                     ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════╝${NC}"
echo "  Branch : $BRANCH"
echo "  SHA    : $SHORT_SHA"
echo "  Total  : ${GATE_TOTAL}s"
echo ""

# ── 护栏 C: dirty-worktree ledger（merge 前确认所有 worktree 的 dirty diff 有归属）──
echo "── dirty-worktree ledger（确认各 worktree 的未提交改动有 PR/task 归属）──"
while IFS= read -r line; do
  wt="$(echo "$line" | sed -n 's/^worktree //p')"
  [ -z "$wt" ] && continue
  dirty="$(git -C "$wt" status --porcelain 2>/dev/null)"
  if [ -n "$dirty" ]; then
    echo -e "${YELLOW}  • $wt 有未提交改动：${NC}"
    echo "$dirty" | head -10
  fi
done < <(git worktree list --porcelain)
echo -e "${GREEN}✓ dirty-worktree ledger 检查完成${NC}"
echo ""

# ── 护栏 D: gate-last-run sentinel（供 check-gate-freshness 判定 gate 是否新鲜）──
GATE_LAST_RUN="$REPO_ROOT/.sounds-great-ai/gate/last-run"
mkdir -p "$(dirname "$GATE_LAST_RUN")"
date -u +"%Y-%m-%dT%H:%M:%SZ" > "$GATE_LAST_RUN"
echo -e "${GREEN}✓ gate-last-run sentinel 已写入: $GATE_LAST_RUN${NC}"
echo ""
echo "可以安全执行 merge-gate 的后续步骤了。"
