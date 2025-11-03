# Makefile for City2TABULA

.PHONY: help build up down logs clean dev create-db extract-features configure

# Default target
help: ## Show this help message
	@echo "City2TABULA Make Commands"
	@echo "========================"
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Docker Environment
build: ## Build the Docker environment
	cd environment && docker compose build

up: ## Start the Docker environment
	cd environment && docker compose up -d

down: ## Stop the Docker environment
	cd environment && docker compose down

logs: ## View Docker logs
	cd environment && docker compose logs -f

status: ## Check container status
	cd environment && docker compose ps

##@ Application Commands
dev: ## Start development environment with shell
	cd environment && docker compose up -d && docker exec -it city2tabula-environment bash

create-db: up ## Create database and setup schemas
	cd environment && docker exec -it city2tabula-environment ./city2tabula -create_db

extract-features: up ## Extract building features
	cd environment && docker exec -it city2tabula-environment ./city2tabula -extract_features

reset-db: up ## Reset the entire database
	cd environment && docker exec -it city2tabula-environment ./city2tabula -reset_all

##@ Complete Workflows
configure: ## Copy docker.env to .env (edit .env file manually for your password)
	@echo "=> Configuring City2TABULA environment..."
	@echo "=> Copying environment configuration..."
	@cp environment/docker.env .env
	@echo "=> .env file created!"
	@echo "=> Please edit .env and update DB_PASSWORD with your PostgreSQL password"
	@echo "   Replace '<your_pg_password>' with your actual password"

setup: build configure ## Build environment, copy .env, and start containers
	@$(MAKE) up
	@echo "✅ Environment is ready! Run 'make dev' to access the shell"
	@echo "📝 Don't forget to edit .env with your PostgreSQL password if you haven't already"

quick-start: setup create-db extract-features ## Complete setup and processing
	@echo "Quick start complete!"

##@ Cleanup
clean: ## Stop containers and remove volumes
	cd environment && docker compose down -v

clean-all: ## Remove containers, volumes, and images
	cd environment && docker compose down -v --rmi all