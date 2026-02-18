.PHONY: help build up down clean logs status test-all demo

# Configuration
COMPOSE_FILE := infrastructure/docker-compose/docker-compose.yml
LB_URL := http://localhost:8090
VIDEO_ID := wolf-1770316220

# Colors for output
BLUE := $(shell printf "\033[0;34m")
GREEN := $(shell printf "\033[0;32m")
YELLOW := $(shell printf "\033[1;33m")
RED := $(shell printf "\033[0;31m")
NC := $(shell printf "\033[0m")

##@ General

help:
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(BLUE)Usage:$(NC)\n  make $(GREEN)<target>$(NC)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(YELLOW)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build & Deploy
build:
	@echo "$(BLUE) Building services...$(NC)"
	@cd infrastructure/docker-compose && docker compose build

up:
	@echo "$(GREEN) Starting...$(NC)"
	@cd infrastructure/docker-compose && docker compose up -d
	@echo "$(GREEN) Started!$(NC)"
	@echo "$(YELLOW) Waiting for services to be ready...$(NC)"
	@sleep 10
	@make status

down:
	@echo "$(RED) Stopping...$(NC)"
	@cd infrastructure/docker-compose && docker compose down

restart: down up

clean:
	@echo "$(RED) Cleaning up...$(NC)"
	@cd infrastructure/docker-compose && docker compose down -v
	@echo "$(GREEN) Cleanup complete$(NC)"

rebuild: clean build up

##@ Status & Logs
status:
	@echo "$(BLUE) Cluster Status$(NC)"
	@echo "---"
	@echo "$(YELLOW) Load Balancer:$(NC)"
	@curl -s http://localhost:8090/health 2>/dev/null | grep -o '"status":"[^"]*"' || echo "  $(RED)DOWN$(NC)"
	@echo ""
	@echo "$(YELLOW) Cache Nodes:$(NC)"
	@for node in 1 2 3; do \
		echo -n "  cache-$$node: "; \
		curl -s "http://localhost:808$$node/coordination/status" 2>/dev/null | grep -o '"state":"[^"]*"' || echo "$(RED)DOWN$(NC)"; \
	done
	@echo ""
	@echo "$(YELLOW) Origin Server:$(NC)"
	@curl -s http://localhost:8443/health 2>/dev/null | grep -o '"status":"[^"]*"' || echo "  $(RED)DOWN$(NC)"
	@echo ""

logs:
	@cd infrastructure/docker-compose && docker compose logs -f

logs-lb:
	@cd infrastructure/docker-compose && docker compose logs -f load-balancer

logs-origin:
	@cd infrastructure/docker-compose && docker compose logs -f origin

logs-cache-1:
	@cd infrastructure/docker-compose && docker compose logs -f cache-1

logs-cache-2:
	@cd infrastructure/docker-compose && docker compose logs -f cache-2

logs-cache-3:
	@cd infrastructure/docker-compose && docker compose logs -f cache-3

logs-cache-all:
	@cd infrastructure/docker-compose && docker compose logs cache-1 cache-2 cache-3

logs-origin-fetch: ## Show origin fetch logs (to verify stampede prevention)
	@echo "$(YELLOW) Origin fetch logs (should show minimal fetches due to caching):$(NC)"
	@cd infrastructure/docker-compose && docker compose logs origin | grep -i "GET /videos" || echo "No origin fetches yet"

logs-cache-locks:
	@echo "$(YELLOW) Lock coordination logs:$(NC)"
	@cd infrastructure/docker-compose && docker compose logs cache-1 cache-2 cache-3 | grep -E "(Got lock|Lock denied|COORDINATED MISS)" || echo "No lock events yet"

logs-cache-gossip:
	@echo "$(YELLOW) Gossip protocol logs:$(NC)"
	@cd infrastructure/docker-compose && docker compose logs cache-1 cache-2 cache-3 | grep -i gossip || echo "No gossip events yet"

##@ Testing
test-basic:
	@echo "$(BLUE) Testing basic video fetch...$(NC)"
	@curl -s -o /dev/null -w "Master playlist: %{http_code} in %{time_total}s\n" $(LB_URL)/hls/$(VIDEO_ID)/master.m3u8
	@curl -s -o /dev/null -w "Segment 0:       %{http_code} in %{time_total}s\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0000.m4s
	@curl -s -o /dev/null -w "Segment 0 (cached): %{http_code} in %{time_total}s\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0000.m4s
	@echo "$(GREEN) Basic test complete!$(NC)"

test-load:
	@echo "$(BLUE) Running load test...$(NC)"
	@bash scripts/load-test.sh $(LB_URL) $(VIDEO_ID) 10 50

test-load-heavy:
	@echo "$(RED) Running HEAVY load test...$(NC)"
	@bash scripts/load-test.sh $(LB_URL) $(VIDEO_ID) 20 100

stampede-test:
	@echo "$(BLUE) Testing stampede prevention...$(NC)"
	@bash scripts/stampede-test.sh $(LB_URL) $(VIDEO_ID) 20

##@ Failure Testing
kill-follower:
	@echo "$(RED) Killing cache-2 (follower)...$(NC)"
	@docker stop telco-cache-2
	@echo "$(YELLOW)Test video fetch:$(NC)"
	@curl -s -o /dev/null -w "Status: %{http_code}, Time: %{time_total}s\n" $(LB_URL)/hls/$(VIDEO_ID)/master.m3u8 || echo "$(RED)Failed!$(NC)"

kill-leader:
	@echo "$(RED) Killing cache-3 (likely leader)...$(NC)"
	@docker stop telco-cache-3
	@echo "$(YELLOW) Waiting for election...$(NC)"
	@sleep 10
	@echo "$(YELLOW) New leader:$(NC)"
	@curl -s http://localhost:8081/coordination/status | grep -o '"state":"[^"]*"' || echo "checking..."
	@curl -s http://localhost:8082/coordination/status | grep -o '"state":"[^"]*"' || echo "checking..."

recover-all:
	@echo "$(GREEN) Recovering all nodes...$(NC)"
	@docker start telco-cache-1 telco-cache-2 telco-cache-3 2>/dev/null || true
	@echo "$(YELLOW)Waiting for cluster to stabilize...$(NC)"
	@sleep 8
	@make status

##@ Latency Testing
# Realistic latency (100-200ms) - simulates origin in another country
latency-realistic: ## Inject 100ms latency to ORIGIN (realistic distant server)
	@echo "$(YELLOW) Simulating origin server in another country (100ms)...$(NC)"
	@bash scripts/inject-latency.sh telco-origin 50 20
	@echo "$(GREEN) Origin now has realistic latency. Cache fetches will be slower!$(NC)"

latency-realistic-high: ## Inject 300ms latency to ORIGIN (high realistic latency)
	@echo "$(YELLOW) Simulating origin server on another continent (300ms)...$(NC)"
	@bash scripts/inject-latency.sh telco-origin 80 30
	@echo "$(GREEN) Origin now has high latency. Cache fetches will be much slower!$(NC)"

latency-remove:
	@echo "$(GREEN) Removing latency from origin...$(NC)"
	@bash scripts/remove-latency.sh telco-origin 2>/dev/null || true
	@echo "$(GREEN) Latency removed!$(NC)"

latency-remove-all:
	@echo "$(GREEN) Removing all latency...$(NC)"
	@bash scripts/remove-latency.sh telco-origin 2>/dev/null || true
	@bash scripts/remove-latency.sh telco-cache-1 2>/dev/null || true
	@bash scripts/remove-latency.sh telco-cache-2 2>/dev/null || true
	@bash scripts/remove-latency.sh telco-cache-3 2>/dev/null || true
	@echo "$(GREEN) All latency removed!$(NC)"

LATENCY_DEMO=80
latency-test:
	@echo "$(BLUE) CDN Latency Test Scenario$(NC)"
	@echo "---"
	@echo "$(YELLOW)1. Baseline - Cached content (FAST):$(NC)"
	@curl -s -o /dev/null -w "  Time: %{time_total}s\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0001.m4s
	@echo ""
	@echo "$(YELLOW)2. Injecting $(LATENCY_DEMO)ms latency to ORIGIN...$(NC)"
	@bash scripts/inject-latency.sh telco-origin $(LATENCY_DEMO) 20 > /dev/null
	@sleep 2
	@echo "$(YELLOW)3. Fetching COLD segment (origin fetch - SLOW):$(NC)"
	@curl -s -o /dev/null -w "  Time: %{time_total}s (includes origin latency!)\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0020.m4s
	@echo "$(YELLOW)4. Fetching SAME segment again (cached - FAST):$(NC)"
	@curl -s -o /dev/null -w "  Time: %{time_total}s (from cache!)\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0020.m4s
	@echo ""
	@echo "$(YELLOW)5. Cleaning up...$(NC)"
	@make latency-remove > /dev/null
	@echo "$(GREEN) Test complete! See how caching improves performance!$(NC)"

##@ Demo
demo: ## Run complete demo scenario
	@bash scripts/demo-scenario.sh $(LB_URL)

demo-setup: ## Setup cluster for demo (run 10 min before presentation)
	@bash scripts/demo-setup.sh

test-all: test-basic test-load stampede-test
	@echo "$(GREEN) All tests complete!$(NC)"
	@echo ""
	@echo "Check logs with: make logs-cache-locks"

##@ Development
shell-cache1: ## Open shell in cache-1 container
	@docker exec -it telco-cache-1 /bin/sh

shell-cache2: ## Open shell in cache-2 container
	@docker exec -it telco-cache-2 /bin/sh

shell-cache3: ## Open shell in cache-3 container
	@docker exec -it telco-cache-3 /bin/sh

shell-lb: ## Open shell in load balancer container
	@docker exec -it telco-lb /bin/sh

shell-origin: ## Open shell in origin container
	@docker exec -it telco-origin /bin/sh

ps: ## Show running containers
	@cd infrastructure/docker-compose && docker compose ps

