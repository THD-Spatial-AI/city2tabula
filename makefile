# Makefile for City2TABULA

.PHONY: help build up down logs clean dev create-db extract-features configure configure-manual

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
configure: ## Interactive configuration: select country and enter password
	@echo "City2TABULA Interactive Configuration"
	@echo "========================================"
	@echo ""
	@echo "Copying base environment configuration..."
	@cp environment/docker.env .env
	@echo "Base configuration copied!"
	@echo ""
	@echo "Available Countries:"
	@echo "======================"
	@echo " 1) austria       - SRID: 31256 (MGI / Austria GK East)"
	@echo " 2) belgium       - SRID: 31370 (Belgian Lambert 72)"
	@echo " 3) cyprus        - SRID: 3879  (GRS 1980 / Cyprus TM)"
	@echo " 4) czechia       - SRID: 5514  (S-JTSK / Krovak East North)"
	@echo " 5) denmark       - SRID: 25832 (ETRS89 / UTM zone 32N)"
	@echo " 6) france        - SRID: 2154  (RGF93 / Lambert-93)"
	@echo " 7) germany       - SRID: 25832 (ETRS89 / UTM zone 32N)"
	@echo " 8) greece        - SRID: 2100  (GGRS87 / Greek Grid)"
	@echo " 9) hungary       - SRID: 23700 (EOV)"
	@echo "10) ireland       - SRID: 29902 (Irish National Grid)"
	@echo "11) italy         - SRID: 3003  (Monte Mario / Italy zone 1)"
	@echo "12) netherlands   - SRID: 28992 (Amersfoort / RD New)"
	@echo "13) norway        - SRID: 25833 (ETRS89 / UTM zone 33N)"
	@echo "14) poland        - SRID: 2180  (ETRS89 / Poland CS2000 zone 5)"
	@echo "15) serbia        - SRID: 3114  (Serbian 1970 / Serbian Grid)"
	@echo "16) slovenia      - SRID: 3794  (Slovenia 1996 / Slovene National Grid)"
	@echo "17) spain         - SRID: 25830 (ETRS89 / UTM zone 30N)"
	@echo "18) sweden        - SRID: 3006  (SWEREF99 TM)"
	@echo "19) united_kingdom - SRID: 27700 (OSGB 1936 / British National Grid)"
	@echo ""
	@read -p "Select country (1-19): " choice; \
	case $$choice in \
		1) COUNTRY="austria"; SRID="31256"; SRS_NAME="MGI / Austria GK East" ;; \
		2) COUNTRY="belgium"; SRID="31370"; SRS_NAME="Belgian Lambert 72" ;; \
		3) COUNTRY="cyprus"; SRID="3879"; SRS_NAME="GRS 1980 / Cyprus TM" ;; \
		4) COUNTRY="czechia"; SRID="5514"; SRS_NAME="S-JTSK / Krovak East North" ;; \
		5) COUNTRY="denmark"; SRID="25832"; SRS_NAME="ETRS89 / UTM zone 32N" ;; \
		6) COUNTRY="france"; SRID="2154"; SRS_NAME="RGF93 / Lambert-93" ;; \
		7) COUNTRY="germany"; SRID="25832"; SRS_NAME="ETRS89 / UTM zone 32N" ;; \
		8) COUNTRY="greece"; SRID="2100"; SRS_NAME="GGRS87 / Greek Grid" ;; \
		9) COUNTRY="hungary"; SRID="23700"; SRS_NAME="EOV" ;; \
		10) COUNTRY="ireland"; SRID="29902"; SRS_NAME="Irish National Grid" ;; \
		11) COUNTRY="italy"; SRID="3003"; SRS_NAME="Monte Mario / Italy zone 1" ;; \
		12) COUNTRY="netherlands"; SRID="28992"; SRS_NAME="Amersfoort / RD New" ;; \
		13) COUNTRY="norway"; SRID="25833"; SRS_NAME="ETRS89 / UTM zone 33N" ;; \
		14) COUNTRY="poland"; SRID="2180"; SRS_NAME="ETRS89 / Poland CS2000 zone 5" ;; \
		15) COUNTRY="serbia"; SRID="3114"; SRS_NAME="Serbian 1970 / Serbian Grid" ;; \
		16) COUNTRY="slovenia"; SRID="3794"; SRS_NAME="Slovenia 1996 / Slovene National Grid" ;; \
		17) COUNTRY="spain"; SRID="25830"; SRS_NAME="ETRS89 / UTM zone 30N" ;; \
		18) COUNTRY="sweden"; SRID="3006"; SRS_NAME="SWEREF99 TM" ;; \
		19) COUNTRY="united_kingdom"; SRID="27700"; SRS_NAME="OSGB 1936 / British National Grid" ;; \
		*) echo "Invalid selection. Using default: germany"; COUNTRY="germany"; SRID="25832"; SRS_NAME="ETRS89 / UTM zone 32N" ;; \
	esac; \
	echo ""; \
	echo "Selected: $$COUNTRY (SRID: $$SRID)"; \
	echo ""; \
	echo "Database Configuration:"; \
	echo "========================="; \
	echo -n "Enter PostgreSQL username [default: postgres]: "; \
	read pg_user; \
	if [ -z "$$pg_user" ]; then pg_user="postgres"; fi; \
	echo -n "Enter PostgreSQL password: "; \
	stty -echo; \
	read pg_password; \
	stty echo; \
	echo ""; \
	echo ""; \
	echo "Updating configuration file..."; \
	sed -i "s/COUNTRY=germany/COUNTRY=$$COUNTRY/" .env; \
	sed -i "s/CITYDB_SRID=25832/CITYDB_SRID=$$SRID/" .env; \
	sed -i "s|CITYDB_SRS_NAME=ETRS89 / UTM zone 32N|CITYDB_SRS_NAME=$$SRS_NAME|" .env; \
	sed -i "s/DB_USER=postgres/DB_USER=$$pg_user/" .env; \
	sed -i "s/<your_pg_password>/$$pg_password/" .env; \
	echo "Configuration completed!"; \
	echo ""; \
	echo "Summary:"; \
	echo "==========="; \
	echo "Country: $$COUNTRY"; \
	echo "SRID: $$SRID"; \
	echo "SRS Name: $$SRS_NAME"; \
	echo "Database: Configured"; \
	echo ""; \
	echo "Next steps:"; \
	echo "- Place your data in data/lod2/$$COUNTRY/ and data/lod3/$$COUNTRY/"
	echo "- Run 'make up' to start containers"; \
	echo "- Run 'make dev' to access development shell"; \


setup: build configure ## Build environment, copy .env, and start containers
	@$(MAKE) up
	@echo "Environment is ready! Run 'make dev' to access the shell"
	@echo "Don't forget to edit .env with your PostgreSQL password if you haven't already"

quick-start: setup create-db extract-features ## Complete setup and processing
	@echo "Quick start complete!"

##@ Cleanup
clean: ## Stop containers and remove volumes
	cd environment && docker compose down -v

clean-all: ## Remove containers, volumes, and images
	cd environment && docker compose down -v --rmi all