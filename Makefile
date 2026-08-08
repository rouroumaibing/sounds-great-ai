SHELL := /bin/bash

.PHONY: dev prod daemon backend frontend build stop clean install upgrade help deep

.DEFAULT_GOAL := help

ifneq ($(filter daemon,$(MAKECMDGOALS)),)
DAEMON_MODE=true
endif

daemon:
	@:

dev:
	@if [ ! -d web/node_modules ]; then \
		echo "Error: web/node_modules not found. Run 'make install' first."; exit 1; \
	elif [ web/package.json -nt web/node_modules/.package-lock.json ]; then \
		echo "Warning: package.json is newer than node_modules. Run 'make install' to update deps."; \
	fi; \
	if [ "$(DAEMON_MODE)" = "true" ]; then \
		mkdir -p .logs .pids; \
		for name in backend frontend; do \
			if [ -f .pids/$$name.pid ] && kill -0 $$(cat .pids/$$name.pid) 2>/dev/null; then \
				echo "Error: $$name already running (PID $$(cat .pids/$$name.pid)). Run 'make stop' first."; exit 1; \
			fi; \
		done; \
		for name in backend frontend; do \
			if [ -f .pids/$$name.pid ] && ! kill -0 $$(cat .pids/$$name.pid) 2>/dev/null; then \
				rm -f .pids/$$name.pid; \
			fi; \
		done; \
		if [ ! -f web/node_modules/.bin/vite ]; then \
			echo "Error: vite not found. Run 'make install' first."; exit 1; \
		fi; \
		go build -o bin/server-dev cmd/server/main.go; \
		if [ $$? -ne 0 ]; then echo "Error: backend build failed"; exit 1; fi; \
		( exec ./bin/server-dev > .logs/backend.log 2>&1 ) & \
		echo $$! > .pids/backend.pid; \
		( cd web && exec ./node_modules/.bin/vite ) > .logs/frontend.log 2>&1 & \
		echo $$! > .pids/frontend.pid; \
		sleep 1; \
		for name in backend frontend; do \
			pid=$$(cat .pids/$$name.pid 2>/dev/null); \
			if ! kill -0 $$pid 2>/dev/null; then \
				echo "Warning: $$name exited immediately, check .logs/$$name.log"; \
			fi; \
		done; \
		echo "Backend PID: $$(cat .pids/backend.pid), Frontend PID: $$(cat .pids/frontend.pid)"; \
		echo "Logs: .logs/backend.log, .logs/frontend.log"; \
		echo "Run 'make stop' to stop."; \
	else \
		trap 'kill 0' EXIT INT TERM; \
		echo "Starting backend on :8080..."; \
		go run cmd/server/main.go & \
		BACKEND_PID=$$!; \
		echo "Starting frontend on :5173..."; \
		cd web && npm run dev & \
		FRONTEND_PID=$$!; \
		echo "Backend PID: $$BACKEND_PID, Frontend PID: $$FRONTEND_PID"; \
		echo "Press Ctrl+C to stop both."; \
		wait; \
	fi

prod:
	@set -e; \
	if [ ! -d web/node_modules ]; then \
		echo "Error: web/node_modules not found. Run 'make install' first."; exit 1; \
	fi; \
	$(MAKE) build; \
	go build -o bin/server cmd/server/main.go; \
	if [ "$(DAEMON_MODE)" = "true" ]; then \
		mkdir -p .logs .pids; \
		for name in backend frontend; do \
			if [ -f .pids/$$name.pid ] && kill -0 $$(cat .pids/$$name.pid) 2>/dev/null; then \
				echo "Error: $$name already running (PID $$(cat .pids/$$name.pid)). Run 'make stop' first."; exit 1; \
			fi; \
		done; \
		for name in backend frontend; do \
			if [ -f .pids/$$name.pid ] && ! kill -0 $$(cat .pids/$$name.pid) 2>/dev/null; then \
				rm -f .pids/$$name.pid; \
			fi; \
		done; \
		nohup ./bin/server > .logs/backend.log 2>&1 & \
		echo $$! > .pids/backend.pid; \
		sleep 1; \
		if ! kill -0 $$(cat .pids/backend.pid) 2>/dev/null; then \
			echo "Warning: backend exited immediately, check .logs/backend.log"; \
		else \
			echo "Server running on :8080 (PID $$(cat .pids/backend.pid))"; \
			echo "Log: .logs/backend.log"; \
			echo "Run 'make stop' to stop."; \
		fi; \
	else \
		echo "Starting production server on :8080..."; \
		./bin/server; \
	fi

backend:
	go run cmd/server/main.go

frontend:
	cd web && npm run dev

install:
	go mod download
	cd web && npm ci

upgrade:
	@read -p "是否需要拉取最新的代码？(y/n) " choice; \
	if [ "$$choice" = "y" ] || [ "$$choice" = "Y" ]; then \
		echo "Pulling latest code..."; git pull; \
	fi; \
	echo "Installing dependencies..."; $(MAKE) install; \
	echo "Building frontend..."; $(MAKE) build; \
	echo "Building Go binary..."; go build -o bin/server cmd/server/main.go; \
	echo "Upgrade complete. Run 'make prod daemon' to restart."

build:
	cd web && npx tsc --noEmit && rm -rf dist && npm run build

stop:
	@found=false; \
	for name in backend frontend; do \
		pidfile=.pids/$$name.pid; \
		if [ -f "$$pidfile" ]; then \
			pid=$$(cat $$pidfile); \
			if kill -0 $$pid 2>/dev/null; then \
				kill $$pid 2>/dev/null || true; \
				sleep 1; \
				if kill -0 $$pid 2>/dev/null; then \
					kill -9 $$pid 2>/dev/null || true; \
					echo "Force-killed $$name (PID $$pid)"; \
				else \
					echo "Stopped $$name (PID was $$pid)"; \
				fi; \
			else \
				echo "$$name PID $$pid already dead (stale pidfile removed)"; \
			fi; \
			rm -f $$pidfile; \
			found=true; \
		fi; \
	done; \
	if [ "$$found" = "false" ]; then \
		echo "No daemon processes found (no PID files in .pids/)"; \
	fi

clean:
	rm -rf web/dist bin/
	rm -f main server

deep: clean
	rm -rf .logs .pids
	rm -f internal/platform/hooks_trace.db internal/platform/hooks_trace.db-wal internal/platform/hooks_trace.db-shm
	find . -name "*.db" -o -name "*.db-wal" -o -name "*.db-shm" 2>/dev/null | grep -v node_modules | grep -v readonly-docs | xargs rm -f 2>/dev/null || true
	rm -rf web/node_modules
	go clean -testcache
	go clean -cache
	@echo "Deep clean complete. Run 'make install' to reinstall dependencies."

help:
	@echo "sounds-great-ai — available commands:"
	@echo ""
	@echo "  make install       Install Go + npm dependencies (npm ci)"
	@echo "  make upgrade       Pull latest code (prompt) + rebuild everything"
	@echo "  make dev           Start backend (:8080) + frontend (:5173) in foreground"
	@echo "  make dev daemon    Start both in background (logs: .logs/, pids: .pids/)"
	@echo "  make prod          Build frontend + compile Go binary + run in foreground"
	@echo "  make prod daemon   Same as prod but in background"
	@echo "  make stop          Stop background processes (via PID files)"
	@echo "  make build         Build frontend for production (tsc + vite build)"
	@echo "  make clean         Remove build artifacts (web/dist, bin/)"
	@echo "  make clean deep    Deep clean: + logs, pids, SQLite, Go cache, node_modules"
	@echo "  make backend       Start Go backend only"
	@echo "  make frontend      Start Vite dev server only"
