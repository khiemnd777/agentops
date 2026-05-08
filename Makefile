COMPOSE ?= docker compose

.DEFAULT_GOAL := help

.PHONY: help up down restart stop log cli host-bridge-start host-bridge-stop host-bridge-log

help:
	@echo "Available targets:"
	@echo "  make up       Start Docker services in the background"
	@echo "  make down     Stop and remove Docker services"
	@echo "  make restart  Restart Docker services"
	@echo "  make stop     Stop Docker services"
	@echo "  make log      Follow Docker service logs"
	@echo "  make cli ARGS=\"...\"  Run AgentOps CLI inside the api container"
	@echo "  make host-bridge-log  Follow AgentOps Host Bridge logs"

up:
	$(COMPOSE) up -d --build
	$(MAKE) host-bridge-start

down:
	$(COMPOSE) down
	$(MAKE) host-bridge-stop

restart:
	$(COMPOSE) down
	$(MAKE) host-bridge-stop
	$(COMPOSE) up -d --build
	$(MAKE) host-bridge-start

stop:
	$(COMPOSE) stop
	$(MAKE) host-bridge-stop

log:
	$(COMPOSE) logs -f

cli:
	$(COMPOSE) exec api /app/agentops-cli $(ARGS)

host-bridge-start:
	@mkdir -p .data
	@set -a; [ ! -f .env ] || . ./.env; set +a; \
	root="$$(pwd)"; \
	port="$${AGENTOPS_HOST_BRIDGE_PORT:-17321}"; \
	pidfile="$$root/.data/host-bridge.pid"; \
	portfile="$$root/.data/host-bridge.port"; \
	logfile="$$root/.data/host-bridge.log"; \
	if [ -f "$$pidfile" ] && kill -0 "$$(cat "$$pidfile")" 2>/dev/null && [ -f "$$portfile" ] && [ "$$(cat "$$portfile")" = "$$port" ]; then \
		echo "AgentOps Host Bridge already running on 127.0.0.1:$$port (pid $$(cat "$$pidfile"))"; \
	else \
		if [ -f "$$pidfile" ] && kill -0 "$$(cat "$$pidfile")" 2>/dev/null; then kill "$$(cat "$$pidfile")" 2>/dev/null || true; fi; \
		rm -f "$$pidfile"; \
		echo "Starting AgentOps Host Bridge on 127.0.0.1:$$port"; \
		(cd "$$root/api" && GOCACHE="$$PWD/.gocache" nohup go run ./cmd/agentops host-bridge --addr "127.0.0.1:$$port" >> "$$logfile" 2>&1 & echo $$! > "$$pidfile"); \
		echo "$$port" > "$$portfile"; \
	fi

host-bridge-stop:
	@pidfile="$$(pwd)/.data/host-bridge.pid"; \
	if [ -f "$$pidfile" ]; then \
		pid="$$(cat "$$pidfile")"; \
		if kill -0 "$$pid" 2>/dev/null; then \
			echo "Stopping AgentOps Host Bridge (pid $$pid)"; \
			kill "$$pid" 2>/dev/null || true; \
		fi; \
		rm -f "$$pidfile"; \
	fi
	@rm -f "$$(pwd)/.data/host-bridge.port"

host-bridge-log:
	@mkdir -p .data
	@touch .data/host-bridge.log
	@tail -f .data/host-bridge.log
