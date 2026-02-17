.PHONY: help build up down clean logs status test-all demo

# Configuration
COMPOSE_FILE := infrastructure/docker-compose/docker-compose.yml
LB_URL := http://localhost:8090
VIDEO_ID := wolf-1770316220

# Colors for output
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[1;33m
RED := \033[0;31m
NC := \033[0m # No Color

##@ General

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n$(BLUE)Usage:$(NC)\n  make $(GREEN)<target>$(NC)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(YELLOW)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Build & Deploy

build: ## Build all services
	@echo "$(BLUE) Building services...$(NC)"
	@cd infrastructure/docker-compose && docker-compose build

up: ## Start all services
	@echo "$(GREEN) Starting...$(NC)"
	@cd infrastructure/docker-compose && docker-compose up -d
	@echo "$(GREEN) Started!$(NC)"
	@echo "$(YELLOW) Waiting for services to be ready...$(NC)"
	@sleep 10
	@make status

down: ## Stop all services
	@echo "$(RED) Stopping...$(NC)"
	@cd infrastructure/docker-compose && docker-compose down

restart: down up ## Restart all services

clean: ## Stop and remove all containers, volumes, and data
	@echo "$(RED) Cleaning up...$(NC)"
	@cd infrastructure/docker-compose && docker-compose down -v
	@echo "$(GREEN) Cleanup complete$(NC)"

rebuild: clean build up ## Clean rebuild and start

##@ Status & Logs

status: ## Show cluster status
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

logs: ## Show logs from all services
	@cd infrastructure/docker-compose && docker-compose logs -f

logs-lb: ## Show load balancer logs
	@cd infrastructure/docker-compose && docker-compose logs -f load-balancer

logs-origin: ## Show origin server logs
	@cd infrastructure/docker-compose && docker-compose logs -f origin

logs-cache-1: ## Show cache-1 logs
	@cd infrastructure/docker-compose && docker-compose logs -f cache-1

logs-cache-2: ## Show cache-2 logs
	@cd infrastructure/docker-compose && docker-compose logs -f cache-2

logs-cache-3: ## Show cache-3 logs
	@cd infrastructure/docker-compose && docker-compose logs -f cache-3

logs-cache-all: ## Show all cache node logs
	@cd infrastructure/docker-compose && docker-compose logs cache-1 cache-2 cache-3

logs-origin-fetch: ## Show origin fetch logs (to verify stampede prevention)
	@echo "$(YELLOW) Origin fetch logs (should show minimal fetches due to caching):$(NC)"
	@cd infrastructure/docker-compose && docker-compose logs origin | grep -i "GET /videos" || echo "No origin fetches yet"

logs-cache-locks: ## Show lock coordination logs
	@echo "$(YELLOW) Lock coordination logs:$(NC)"
	@cd infrastructure/docker-compose && docker-compose logs cache-1 cache-2 cache-3 | grep -E "(Got lock|Lock denied|COORDINATED MISS)" || echo "No lock events yet"

logs-cache-gossip: ## Show gossip protocol logs
	@echo "$(YELLOW) Gossip protocol logs:$(NC)"
	@cd infrastructure/docker-compose && docker-compose logs cache-1 cache-2 cache-3 | grep -i gossip || echo "No gossip events yet"

##@ Testing

test-basic: ## Test basic video fetch through load balancer
	@echo "$(BLUE) Testing basic video fetch...$(NC)"
	@curl -s -o /dev/null -w "Master playlist: %{http_code} in %{time_total}s\n" $(LB_URL)/hls/$(VIDEO_ID)/master.m3u8
	@curl -s -o /dev/null -w "Segment 0:       %{http_code} in %{time_total}s\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0000.m4s
	@curl -s -o /dev/null -w "Segment 0 (cached): %{http_code} in %{time_total}s\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0000.m4s
	@echo "$(GREEN) Basic test complete!$(NC)"

test-load: ## Run load test (10 concurrent users, 50 requests)
	@echo "$(BLUE) Running load test...$(NC)"
	@bash scripts/load-test.sh $(LB_URL) $(VIDEO_ID) 10 50

test-load-heavy: ## Run heavy load test (20 concurrent users, 100 requests)
	@echo "$(RED) Running HEAVY load test...$(NC)"
	@bash scripts/load-test.sh $(LB_URL) $(VIDEO_ID) 20 100

stampede-test: ## Test cache stampede prevention (lock coordination)
	@echo "$(BLUE) Testing stampede prevention...$(NC)"
	@bash scripts/stampede-test.sh $(LB_URL) $(VIDEO_ID) 20

##@ Failure Testing
kill-follower: ## Kill cache-2 (follower node)
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
	@bash scripts/inject-latency.sh telco-origin 100 20
	@echo "$(GREEN) Origin now has realistic latency. Cache fetches will be slower!$(NC)"

latency-realistic-high: ## Inject 300ms latency to ORIGIN (high realistic latency)
	@echo "$(YELLOW) Simulating origin server on another continent (300ms)...$(NC)"
	@bash scripts/inject-latency.sh telco-origin 300 50
	@echo "$(GREEN) Origin now has high latency. Cache fetches will be much slower!$(NC)"

# Demo latency (2-3 seconds) - clearly visible for presentations!
latency-demo: ## Inject 2s latency to ORIGIN (DEMO - very visible!)
	@echo "$(RED) DEMO MODE: Simulating VERY distant origin (2s delay)...$(NC)"
	@bash scripts/inject-latency.sh telco-origin 2000 200
	@echo "$(GREEN) Origin now has 2s delay - perfect for demos!$(NC)"
	@echo "$(YELLOW) Try fetching a cold segment to see the delay!$(NC)"

latency-demo-extreme: ## Inject 3s latency to ORIGIN (EXTREME - impossible to miss!)
	@echo "$(RED) DEMO MODE: Simulating EXTREMELY distant origin (3s delay)...$(NC)"
	@bash scripts/inject-latency.sh telco-origin 3000 300
	@echo "$(GREEN) Origin now has 3s delay - professor will definitely see this!$(NC)"
	@echo "$(YELLOW) Cache fetches take 3+ seconds, but cached content is instant!$(NC)"

latency-remove: ## Remove latency from origin
	@echo "$(GREEN) Removing latency from origin...$(NC)"
	@bash scripts/remove-latency.sh telco-origin 2>/dev/null || true
	@echo "$(GREEN) Latency removed!$(NC)"

latency-remove-all: ## Remove latency from all containers (cleanup)
	@echo "$(GREEN) Removing all latency...$(NC)"
	@bash scripts/remove-latency.sh telco-origin 2>/dev/null || true
	@bash scripts/remove-latency.sh telco-cache-1 2>/dev/null || true
	@bash scripts/remove-latency.sh telco-cache-2 2>/dev/null || true
	@bash scripts/remove-latency.sh telco-cache-3 2>/dev/null || true
	@echo "$(GREEN) All latency removed!$(NC)"

latency-test: ## Run realistic latency test (100ms origin delay)
	@echo "$(BLUE) CDN Latency Test Scenario$(NC)"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "$(YELLOW)1. Baseline - Cached content (FAST):$(NC)"
	@curl -s -o /dev/null -w "  Time: %{time_total}s\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0001.m4s
	@echo ""
	@echo "$(YELLOW)2. Injecting 100ms latency to ORIGIN...$(NC)"
	@bash scripts/inject-latency.sh telco-origin 100 20 > /dev/null
	@sleep 2
	@echo "$(YELLOW)3. Fetching COLD segment (origin fetch - SLOW):$(NC)"
	@curl -s -o /dev/null -w "  Time: %{time_total}s (includes origin latency!)\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0020.m4s
	@echo "$(YELLOW)4. Fetching SAME segment again (cached - FAST):$(NC)"
	@curl -s -o /dev/null -w "  Time: %{time_total}s (from cache!)\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0020.m4s
	@echo ""
	@echo "$(YELLOW)5. Cleaning up...$(NC)"
	@make latency-remove > /dev/null
	@echo "$(GREEN) Test complete! See how caching improves performance!$(NC)"

latency-demo-test: ## Run DEMO latency test (2.5s origin delay - VERY visible!)
	@echo "$(RED) CDN DEMO - Clearly Visible Latency!$(NC)"
	@echo "---"
	@echo "$(YELLOW)1. Cached content (instant):$(NC)"
	@curl -s -o /dev/null -w "  Time: %{time_total}s (from cache!)\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0003.m4s
	@echo ""
	@echo "$(RED)2. Injecting 2.5s DEMO latency to ORIGIN...$(NC)"
	@bash scripts/inject-latency.sh telco-origin 2500 250 > /dev/null
	@sleep 2
	@echo ""
	@echo "$(YELLOW)3. Fetching from DISTANT origin - watch the delay!$(NC)"
	@curl -s -o /dev/null -w "  Time: %{time_total}s (origin is FAR away!)\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0021.m4s
	@echo ""
	@echo "$(YELLOW)4. Same segment from cache - instant!$(NC)"
	@curl -s -o /dev/null -w "  Time: %{time_total}s (cached = fast!)\n" $(LB_URL)/hls/$(VIDEO_ID)/segment_0021.m4s
	@echo ""
	@echo "$(YELLOW)5. Cleaning up...$(NC)"
	@make latency-remove > /dev/null
	@echo "$(GREEN) Demo complete! CDN value is crystal clear!$(NC)"

##@ Demo

demo: ## Run complete demo scenario (impresses professors!)
	@bash scripts/demo-scenario.sh $(LB_URL)

demo-quick: ## Quick demo (health + basic fetch + failure)
	@echo "$(BLUE) QUICK DEMO$(NC)"
	@echo "---"
	@echo ""
	@make status
	@echo ""
	@echo "$(YELLOW)Testing video fetch...$(NC)"
	@make test-basic
	@echo ""
	@echo "$(YELLOW)Simulating node failure...$(NC)"
	@make kill-follower
	@echo ""
	@echo "$(YELLOW)Recovering...$(NC)"
	@make recover-all
	@echo ""
	@echo "$(GREEN) Quick demo complete!$(NC)"

test-all: test-basic test-load stampede-test ## Run all tests in sequence
	@echo "$(GREEN) All tests complete!$(NC)"
	@echo ""
	@echo "$(YELLOW)Summary:$(NC)"
	@echo "  ✓ Basic functionality"
	@echo "  ✓ Load handling"
	@echo "  ✓ Stampede prevention"
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
	@cd infrastructure/docker-compose && docker-compose ps

