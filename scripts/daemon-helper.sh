#!/usr/bin/env bash
# Cross-platform helpers for the sounds-great-ai Makefile daemon lifecycle.
#
# Used by `make dev daemon` / `make prod daemon` / `make stop` to:
#   - detect what (if anything) is listening on a TCP port
#   - tell whether a given PID belongs to OUR project binaries
#   - gracefully kill a PID (SIGTERM, wait, SIGKILL)
#   - pre-flight a port: free = ok; ours = reclaim; foreign/legacy = refuse
#
# Our binaries are matched by command line containing "sounds-great-ai"
# (covers sounds-great-ai, sounds-great-ai-dev, and the .exe Windows variant).
# We deliberately do NOT match the generic name "server": that would risk
# killing another project's `bin/server` process occupying the same port.
#
# Supported OS: macOS (Darwin), Linux, Windows (Git Bash / MSYS / Cygwin).

os_detect() {
  case "$(uname -s 2>/dev/null)" in
    Darwin*) echo mac ;;
    Linux*)  echo linux ;;
    MINGW*|MSYS*|CYGWIN*) echo windows ;;
    *) echo unknown ;;
  esac
}

# port_pid <port>  -> print one PID per line listening on TCP <port>, or nothing.
port_pid() {
  local port="$1" os; os="$(os_detect)"
  case "$os" in
    mac)
      lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null
      ;;
    linux)
      if command -v lsof >/dev/null 2>&1; then
        lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null
      else
        # fallback: ss -ltnp, extract pid=NNN from the process column
        ss -ltnp 2>/dev/null | awk -v p=":$port" '$4 ~ p {
          if (match($0, /pid=([0-9]+)/, a)) print a[1]
        }'
      fi
      ;;
    windows)
      # netstat -ano: last column is the PID. Keep LISTENING rows whose local
      # address ends with :port (handles both 0.0.0.0:port and [::]:port).
      netstat -ano -p TCP 2>/dev/null | awk -v p=":$port" '
        tolower($4) ~ /listen/ && $2 ~ p { print $NF }
      ' | sort -u
      ;;
    *) ;;
  esac
}

# proc_cmd <pid>  -> print the command line / executable path of <pid>, or nothing.
proc_cmd() {
  local pid="$1" os; os="$(os_detect)"
  case "$os" in
    mac|linux)
      local out
      out="$(ps -p "$pid" -o command= 2>/dev/null)"
      if [ -z "$out" ]; then
        # Fallback: lsof -p reveals the executable (txt) path when ps is
        # restricted (e.g. some sandboxes). The exe is the line typed "txt".
        out="$(lsof -p "$pid" 2>/dev/null | awk '$4=="txt" {print $NF; exit}')"
      fi
      echo "$out"
      ;;
    windows)   tasklist /fi "PID eq $pid" /fo csv /nh 2>/dev/null | head -1 ;;
    *) ;;
  esac
}

# is_ours <pid>  -> exit 0 if PID belongs to our project binaries, else 1.
is_ours() {
  local pid="$1" cmd; cmd="$(proc_cmd "$pid")"
  [ -z "$cmd" ] && return 1
  case "$cmd" in
    *sounds-great-ai*) return 0 ;;
    *) return 1 ;;
  esac
}

# kill_pid <pid>  -> SIGTERM, wait up to ~2.5s, then SIGKILL if still alive.
kill_pid() {
  local pid="$1"
  kill "$pid" 2>/dev/null || return 0
  local i
  for i in 1 2 3 4 5; do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 0.5
  done
  kill -9 "$pid" 2>/dev/null || true
}

# ensure_port <port> [reclaim]
#   reclaim=1: if a port is held by OUR process, kill it and return 0.
#   reclaim=0: never auto-kill (e.g. the frontend port, which runs plain vite).
# Returns 0 when the port is free (or was reclaimed); 1 when it must be
# freed manually (foreign process, or legacy generic "server" build).
ensure_port() {
  local port="$1" reclaim="${2:-0}" pid cmd
  for pid in $(port_pid "$port"); do
    cmd="$(proc_cmd "$pid")"
    if is_ours "$pid"; then
      if [ "$reclaim" = "1" ]; then
        echo "Reclaiming our stale process (PID $pid) on :$port"
        echo "  cmd: $cmd"
        kill_pid "$pid"
      else
        echo "Port :$port is held by our process (PID $pid). Run 'make stop' first."
        echo "  cmd: $cmd"
        return 1
      fi
    else
      # Not ours. If it looks like our OLD generic-named build, give a precise hint.
      if echo "$cmd" | grep -qi "bin/server"; then
        echo "Port :$port is held by a LEGACY 'server' build (PID $pid)."
        echo "  cmd: $cmd"
        echo "  This is an old sounds-great-ai binary. Kill it manually, e.g.:"
        echo "    lsof -tiTCP:$port | xargs kill -9"
        echo "  (We no longer match the generic name 'server' to avoid killing other projects.)"
      else
        echo "Port :$port is held by a FOREIGN process (PID $pid)."
        echo "  cmd: $cmd"
        echo "  Free it yourself or change the port before running 'make dev daemon'."
      fi
      return 1
    fi
  done
  return 0
}

main() {
  local cmd="$1"; shift || true
  case "$cmd" in
    port_pid|proc_cmd|is_ours|kill_pid|ensure_port) "$cmd" "$@" ;;
    *) echo "usage: $0 {port_pid|proc_cmd|is_ours|kill_pid|ensure_port} ..." >&2; exit 2 ;;
  esac
}

main "$@"
