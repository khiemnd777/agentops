COMPOSE ?= docker compose

.DEFAULT_GOAL := help

.PHONY: help up down restart stop log cli

help:
	@echo "Available targets:"
	@echo "  make up       Start Docker services in the background"
	@echo "  make down     Stop and remove Docker services"
	@echo "  make restart  Restart Docker services"
	@echo "  make stop     Stop Docker services"
	@echo "  make log      Follow Docker service logs"
	@echo "  make cli ARGS=\"...\"  Run AgentOps CLI inside the api container"

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down

restart:
	$(COMPOSE) down
	$(COMPOSE) up -d --build

stop:
	$(COMPOSE) stop

log:
	$(COMPOSE) logs -f

cli:
	$(COMPOSE) exec api /app/agentops-cli $(ARGS)
