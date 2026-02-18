.PHONY: help build

# Colors for output
BLUE := $(shell printf "\033[0;34m")
GREEN := $(shell printf "\033[0;32m")
YELLOW := $(shell printf "\033[1;33m")
RED := $(shell printf "\033[0;31m")
NC := $(shell printf "\033[0m") # No Color

# Configuration
TOPO := infrastructure/containerlab/topology.yml
VIDEO_ID := wolf-1770316220

##@ Help
help:
	@echo ""
	@echo "$(BLUE)Telco CDN - MEC Oulu$(NC)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "  $(GREEN)%-20s$(NC) %s\n", "make <target>", ""} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(YELLOW)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo ""

##@ Build
build: ## Build Docker images
	@echo "$(BLUE)Building images...$(NC)"
	docker build -t telco-cdn-origin:latest -f services/origin/Dockerfile services/origin
	docker build -t telco-cdn-cache:latest -f services/cache-node/Dockerfile services/cache-node
	docker build -t telco-cdn-lb:latest -f services/load-balancer/Dockerfile services/load-balancer
	@echo "$(GREEN)Build complete!$(NC)"

##@ Containerlab
clab-install: ## Install containerlab
	@bash scripts/install-containerlab.sh

clab-up: ## Deploy MEC Oulu topology
	@echo "$(BLUE)Deploying MEC Oulu...$(NC)"
	sudo containerlab deploy -t $(TOPO) --reconfigure
	@echo "$(YELLOW)Waiting for containers...$(NC)"
	@sleep 10
	@echo "$(YELLOW)Applying network latencies...$(NC)"
	@sudo bash scripts/apply-latency.sh
	@echo ""
	@echo "$(GREEN)MEC Oulu is ready!$(NC)"
	@echo ""
	@echo "Access:"
	@echo "  LB:      http://localhost:8080"
	@echo "  Origin:  http://localhost:8081"
	@echo "  Cache-1: http://localhost:8001"
	@echo "  Cache-2: http://localhost:8002"
	@echo "  Cache-3: http://localhost:8003"
	@echo ""
	@echo "Verify latencies: make test-latency"
	@echo ""

clab-down: ## Destroy MEC topology
	@echo "$(RED)Destroying MEC Oulu...$(NC)"
	@sudo containerlab destroy -t $(TOPO) --cleanup
	@echo "$(GREEN)Destroyed!$(NC)"

clab-inspect: ## Show topology status
	@sudo containerlab inspect -t $(TOPO)

clab-graph: ## Open topology graph
	@sudo containerlab graph -t $(TOPO)

##@ Testing
test-fetch: ## Test video fetch
	@echo "$(BLUE)Testing video fetch...$(NC)"
	@curl -s -o /dev/null -w "Master: %{http_code} in %{time_total}s\n" http://localhost:8080/hls/$(VIDEO_ID)/master.m3u8
	@curl -s -o /dev/null -w "Seg 0:  %{http_code} in %{time_total}s\n" http://localhost:8080/hls/$(VIDEO_ID)/segment_0000.m4s
	@curl -s -o /dev/null -w "Cached: %{http_code} in %{time_total}s\n" http://localhost:8080/hls/$(VIDEO_ID)/segment_0000.m4s

status: ## Snapshot: containers, health, leader, load, latency
	@echo ""
	@echo "$(BLUE)[1] Core health (LB & Origin)$(NC)"
	@curl -s http://localhost:8080/health || echo "LB down"
	@echo ""
	@curl -s http://localhost:8081/health || echo "Origin down"
	@echo ""
	@echo "$(BLUE)[2] Cache coordination status (leader info)$(NC)"
	@echo "Cache-1:" && curl -s http://localhost:8001/coordination/status || echo "Cache-1 down"
	@echo "" && echo "Cache-2:" && curl -s http://localhost:8002/coordination/status || echo "Cache-2 down"
	@echo "" && echo "Cache-3:" && curl -s http://localhost:8003/coordination/status || echo "Cache-3 down"
	@echo ""

test-latency: ## Real HTTP miss vs hit test
	@-docker exec oulu-telco-cdn-oulu-oulu-cache-1 rm -rf /app/data/* 2>/dev/null || true
	@-docker exec oulu-telco-cdn-oulu-oulu-cache-2 rm -rf /app/data/* 2>/dev/null || true
	@-docker exec oulu-telco-cdn-oulu-oulu-cache-3 rm -rf /app/data/* 2>/dev/null || true
	@sleep 2
	@curl -s -o /dev/null -w "%{time_total}s\n" http://localhost:8080/hls/$(VIDEO_ID)/segment_0000.m4s

test-election: ## Check leader election
	@echo "$(BLUE)Leader election status:$(NC)"
	@echo ""
	@echo "Cache 1:"
	@docker logs oulu-telco-cdn-oulu-oulu-cache-1 2>&1 | grep -i election | tail -3 || echo "  No logs"
	@echo ""
	@echo "Cache 2:"
	@docker logs oulu-telco-cdn-oulu-oulu-cache-2 2>&1 | grep -i election | tail -3 || echo "  No logs"
	@echo ""
	@echo "Cache 3:"
	@docker logs oulu-telco-cdn-oulu-oulu-cache-3 2>&1 | grep -i election | tail -3 || echo "  No logs"

test-gossip: ## Check gossip protocol
	@echo "$(BLUE)Gossip logs:$(NC)"
	@docker logs oulu-telco-cdn-oulu-oulu-cache-1 2>&1 | grep -i gossip | tail -5 || echo "No gossip logs"

test-stampede: ## Test cache stampede protection (concurrent requests)
	@echo "$(BLUE)Testing cache stampede protection...$(NC)"
	@echo ""
	@echo "Clearing cache first..."
	@-docker exec oulu-telco-cdn-oulu-oulu-cache-1 rm -rf /app/data/* 2>/dev/null || true
	@-docker exec oulu-telco-cdn-oulu-oulu-cache-2 rm -rf /app/data/* 2>/dev/null || true
	@-docker exec oulu-telco-cdn-oulu-oulu-cache-3 rm -rf /app/data/* 2>/dev/null || true
	@sleep 2
	@echo ""
	@echo "Sending 10 concurrent requests for same segment..."
	@for i in $$(seq 1 10); do \
		(curl -s -o /dev/null -w "Request $$i: %{http_code} in %{time_total}s\n" \
			http://localhost:8080/hls/$(VIDEO_ID)/segment_0005.m4s &); \
	done; \
	wait
	@echo ""
	@echo "$(GREEN)Check logs for distributed lock activity:$(NC)"
	@docker logs oulu-telco-cdn-oulu-oulu-cache-1 2>&1 | grep -i -E "(lock|stampede|coordination)" | tail -5 || echo "  No lock logs (may not be implemented)"

##@ Network
network-apply: ## Apply network latencies using containerlab netem
	@echo "$(BLUE)Applying network latencies...$(NC)"
	@sudo bash scripts/apply-latency.sh

network-remove: ## Remove network latencies
	@echo "$(BLUE)Removing network latencies...$(NC)"
	@sudo bash scripts/remove-latency.sh

network-show: ## Show current network impairments
	@echo "$(BLUE)Current network configuration:$(NC)"
	@sudo bash scripts/show-latency.sh

##@ Logs
logs-lb: ## View LB logs
	@docker logs -f oulu-telco-cdn-oulu-oulu-lb

logs-cache1: ## View cache-1 logs
	@docker logs -f oulu-telco-cdn-oulu-oulu-cache-1

logs-cache2: ## View cache-2 logs
	@docker logs -f oulu-telco-cdn-oulu-oulu-cache-2

logs-cache3: ## View cache-3 logs
	@docker logs -f oulu-telco-cdn-oulu-oulu-cache-3

logs-origin: ## View origin logs
	@docker logs -f oulu-telco-cdn-oulu-origin

logs-all: ## View all logs
	@docker logs oulu-telco-cdn-oulu-oulu-lb & \
	docker logs oulu-telco-cdn-oulu-oulu-cache-1 & \
	docker logs oulu-telco-cdn-oulu-oulu-cache-2 & \
	docker logs oulu-telco-cdn-oulu-oulu-cache-3

##@ Debug
shell-lb: ## Shell into LB
	@docker exec -it oulu-telco-cdn-oulu-oulu-lb sh

shell-cache1: ## Shell into cache-1
	@docker exec -it oulu-telco-cdn-oulu-oulu-cache-1 sh

shell-cache2: ## Shell into cache-2
	@docker exec -it oulu-telco-cdn-oulu-oulu-cache-2 sh

shell-cache3: ## Shell into cache-3
	@docker exec -it oulu-telco-cdn-oulu-oulu-cache-3 sh

shell-client: ## Shell into client
	@docker exec -it oulu-telco-cdn-oulu-client sh

ps: ## Show containers
	@docker ps --filter "name=oulu-telco-cdn-oulu" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

##@ Demo
demo-1: ## Demo: Basic fetch
	@echo "$(BLUE)Demo 1: MEC serving content$(NC)"
	@echo ""
	@echo "Fetching from MEC LB..."
	@time curl -s http://localhost:8080/hls/$(VIDEO_ID)/master.m3u8 -o /tmp/demo.m3u8
	@echo ""
	@echo "Content:"
	@head -10 /tmp/demo.m3u8
	@rm /tmp/demo.m3u8

demo-2: ## Demo: Cache performance
	@echo "$(BLUE)Demo 2: Cache hit performance$(NC)"
	@echo ""
	@echo "First fetch (cache miss):"
	@curl -s -o /dev/null -w "Time: %{time_total}s\n" http://localhost:8080/hls/$(VIDEO_ID)/segment_0010.m4s
	@echo ""
	@echo "Second fetch (cache hit):"
	@curl -s -o /dev/null -w "Time: %{time_total}s\n" http://localhost:8080/hls/$(VIDEO_ID)/segment_0010.m4s

demo-3: ## Demo: Leader election
	@echo "$(BLUE)Demo 3: Leader election$(NC)"
	@echo ""
	@echo "Current state:"
	@make test-election
	@echo ""
	@echo "$(YELLOW)Stopping cache-1...$(NC)"
	@docker stop oulu-telco-cdn-oulu-oulu-cache-1
	@sleep 5
	@echo ""
	@echo "New state:"
	@make test-election
	@echo ""
	@echo "$(GREEN)Restarting cache-1...$(NC)"
	@docker start oulu-telco-cdn-oulu-oulu-cache-1

demo-client: ## Run web client against MEC LB
	@echo "$(BLUE)Starting React client (frontend)$(NC)"
	@cd services/client/frontend && npm install --silent && npm run dev -- --host 0.0.0.0 --port 5173

##@ Cleanup
clean: clab-down ## Full cleanup (destroy containerlab topology)

clean-docker: ## Stop and remove all telco containers (incl. docker-compose)
	@echo "$(RED)Cleaning up all telco containers...$(NC)"
	@docker ps -a --filter "name=telco" --format "{{.Names}}" | xargs -r docker rm -f
	@docker ps -a --filter "name=oulu-telco" --format "{{.Names}}" | xargs -r docker rm -f
	@echo "$(GREEN)Cleanup complete!$(NC)"

clean-all: clab-down clean-docker ## Nuclear cleanup (everything)
	@echo "$(RED)Removing networks...$(NC)"
	@docker network prune -f
	@echo "$(GREEN)All clean!$(NC)"

.DEFAULT_GOAL := help
