.PHONY: help build build-ml build-fl-client build-all

# Colors for output
BLUE := $(shell printf "\033[0;34m")
GREEN := $(shell printf "\033[0;32m")
YELLOW := $(shell printf "\033[1;33m")
RED := $(shell printf "\033[0;31m")
NC := $(shell printf "\033[0m")

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
build: ## Build core Docker images (origin, cache, lb)
	@echo "$(BLUE)Building core images...$(NC)"
	docker build -t telco-cdn-origin:latest -f services/origin/Dockerfile services/origin
	docker build -t telco-cdn-cache:latest -f services/cache-node/Dockerfile services/cache-node
	docker build -t telco-cdn-lb:latest -f services/load-balancer/Dockerfile services/load-balancer
	@echo "$(GREEN)Core images built!$(NC)"

build-ml: ## Build ML aggregator image
	@echo "$(BLUE)Building ML aggregator...$(NC)"
	docker build --no-cache -t telco-cdn-ml:latest -f ml/aggregator/Dockerfile ml/aggregator
	@echo "$(GREEN)ML aggregator built!$(NC)"

build-fl-client: ## Build FL client image
	@echo "$(BLUE)Building FL client...$(NC)"
	docker build --no-cache -t edge-fl-client:latest -f ml/fl.Dockerfile ml
	@echo "$(GREEN)FL client built!$(NC)"

##@ Data Directories
dirs: ## Create required data directories
	@echo "$(BLUE)Creating data directories...$(NC)"
	@mkdir -p data/logs/cache-1 data/logs/cache-2 data/logs/cache-3
	@mkdir -p data/oulu-logs data/models
	@mkdir -p data/origin-uploads data/origin-hls
	@mkdir -p data/cache-1 data/cache-2 data/cache-3
	@touch data/videos.json
	@cp ml/aggregator/models/best_xgb_model.pkl data/models/ 2>/dev/null || true
	@cp ml/aggregator/models/scaler.pkl data/models/ 2>/dev/null || true
	@echo "$(GREEN)Directories created!$(NC)"

build-all: build build-ml build-fl-client dirs ## Build all images and create directories

##@ Containerlab
clab-install: ## Install containerlab
	@bash scripts/install-containerlab.sh

clab-up: dirs ## Deploy MEC Oulu topology
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
	@echo "  LB:         http://localhost:8080"
	@echo "  Origin:     http://localhost:8081"
	@echo "  Cache-1:    http://localhost:8001"
	@echo "  Cache-2:    http://localhost:8002"
	@echo "  Cache-3:    http://localhost:8003"
	@echo "  ML Service: http://localhost:8092"
	@echo "  Prometheus: http://localhost:9090"
	@echo "  Grafana:    http://localhost:3000"
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

test-logs: ## Verify NDJSON logs are being written
	@echo "$(BLUE)Checking access logs...$(NC)"
	@echo "Cache-1 logs:"
	@cat data/logs/cache-1/access_cache-1.ndjson 2>/dev/null | head -3 || echo "  No logs yet"
	@echo "Cache-2 logs:"
	@cat data/logs/cache-2/access_cache-2.ndjson 2>/dev/null | head -3 || echo "  No logs yet"
	@echo "Cache-3 logs:"
	@cat data/logs/cache-3/access_cache-3.ndjson 2>/dev/null | head -3 || echo "  No logs yet"

status: ## Snapshot: containers, health, leader, load, latency
	@echo ""
	@echo "$(BLUE)[1] Core health (LB & Origin)$(NC)"
	@curl -s http://localhost:8080/health || echo "LB down"
	@echo ""
	@curl -s http://localhost:8081/health || echo "Origin down"
	@echo ""
	@echo "$(BLUE)[2] Cache coordination status$(NC)"
	@echo "Cache-1:" && curl -s http://localhost:8001/coordination/status || echo "Cache-1 down"
	@echo "" && echo "Cache-2:" && curl -s http://localhost:8002/coordination/status || echo "Cache-2 down"
	@echo "" && echo "Cache-3:" && curl -s http://localhost:8003/coordination/status || echo "Cache-3 down"
	@echo ""
	@echo "$(BLUE)[3] ML Aggregator$(NC)"
	@curl -s http://localhost:8092/health || echo "ML service down"
	@echo ""
	@echo "$(BLUE)[4] Monitoring$(NC)"
	@curl -s http://localhost:9090/-/healthy || echo "Prometheus down"
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

test-stampede: ## Test cache stampede protection
	@echo "$(BLUE)Testing cache stampede protection...$(NC)"
	@-docker exec oulu-telco-cdn-oulu-oulu-cache-1 rm -rf /app/data/* 2>/dev/null || true
	@-docker exec oulu-telco-cdn-oulu-oulu-cache-2 rm -rf /app/data/* 2>/dev/null || true
	@-docker exec oulu-telco-cdn-oulu-oulu-cache-3 rm -rf /app/data/* 2>/dev/null || true
	@sleep 2
	@echo "Sending 10 concurrent requests for same segment..."
	@for i in $$(seq 1 10); do \
		(curl -s -o /dev/null -w "Request $$i: %{http_code} in %{time_total}s\n" \
			http://localhost:8080/hls/$(VIDEO_ID)/segment_0005.m4s &); \
	done; \
	wait

test-load-grafana: ## Generate continuous traffic for Grafana visualization
	@echo "$(BLUE)Generating test traffic for Grafana...$(NC)"
	@for i in $$(seq 1 50); do \
		curl -s -o /dev/null http://localhost:8080/hls/$(VIDEO_ID)/master.m3u8; \
		curl -s -o /dev/null http://localhost:8080/hls/$(VIDEO_ID)/segment_0001.m4s; \
		curl -s -o /dev/null http://localhost:8080/hls/$(VIDEO_ID)/segment_0002.m4s; \
		curl -s -o /dev/null http://localhost:8080/hls/$(VIDEO_ID)/segment_0003.m4s; \
		sleep 0.5; \
	done
	@echo "$(GREEN)Done!$(NC)"

##@ FL Aggregator
check-fl-status: ## Check FL aggregator status and metrics
	@echo "$(BLUE)FL Aggregator Status...$(NC)"
	@curl -s http://localhost:8092/status | python3 -m json.tool
	@echo "\n$(BLUE)FL Prometheus Metrics...$(NC)"
	@curl -s http://172.26.26.11:8092/metrics | grep -E "fl_avg|fl_rounds|fl_participating"
	
##@ Network
network-apply: ## Apply network latencies
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

logs-ml: ## View ML aggregator logs
	@docker logs -f oulu-telco-cdn-oulu-ml-service

logs-fl: ## View FL client logs
	@docker logs oulu-telco-cdn-oulu-oulu-fl-client-1
	@docker logs oulu-telco-cdn-oulu-oulu-fl-client-2
	@docker logs oulu-telco-cdn-oulu-oulu-fl-client-3

logs-all: ## View all logs
	@docker logs oulu-telco-cdn-oulu-oulu-lb & \
	docker logs oulu-telco-cdn-oulu-oulu-cache-1 & \
	docker logs oulu-telco-cdn-oulu-oulu-cache-2 & \
	docker logs oulu-telco-cdn-oulu-oulu-cache-3

##@ FL Monitoring
fl-status: ## Show FL aggregator status
	@echo "$(BLUE)FL Status:$(NC)"
	@curl -s http://localhost:8092/status | python -m json.tool

fl-metrics: ## Show FL Prometheus metrics
	@echo "$(BLUE)FL Prometheus metrics:$(NC)"
	@curl -s http://localhost:8092/metrics

fl-logs: ## Show FL logs
	@echo "$(BLUE)ML Aggregator:$(NC)"
	@docker logs oulu-telco-cdn-oulu-ml-service 2>&1 | tail -20
	@echo ""
	@echo "$(BLUE)FL Client:$(NC)"
	@docker logs oulu-telco-cdn-oulu-oulu-fl-client 2>&1 | tail -20

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

shell-ml: ## Shell into ML service
	@docker exec -it oulu-telco-cdn-oulu-ml-service sh

ps: ## Show containers
	@docker ps --filter "name=oulu-telco-cdn-oulu" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

##@ Evaluation
eval-all: ## Run complete evaluation suite
	@chmod +x scripts/*.sh
	@bash scripts/run-all-tests.sh

eval-baseline: ## Test baseline performance (cold vs warm cache)
	@chmod +x scripts/test-baseline.sh
	@bash scripts/test-baseline.sh

eval-cache-hit: ## Test cache hit ratio with Zipf distribution
	@chmod +x scripts/test-cache-hit.sh
	@bash scripts/test-cache-hit.sh

eval-election: ## Test leader election and failover
	@chmod +x scripts/test-election.sh
	@bash scripts/test-election.sh

eval-load: ## Run k6 load test
	@echo "$(BLUE)Running k6 load test...$(NC)"
	@cd benchmarks/load-testing && k6 run --out json=results.json k6-test.js

eval-analyze: ## Analyze results and generate graphs
	@echo "$(BLUE)Analyzing results...$(NC)"
	@python3 scripts/analyze-results.py

eval-collect: ## Collect metrics from Prometheus
	@chmod +x scripts/collect-metrics.sh
	@bash scripts/collect-metrics.sh 60

##@ Demo
demo-1: ## Demo: Basic fetch
	@echo "$(BLUE)Demo 1: MEC serving content$(NC)"
	@time curl -s http://localhost:8080/hls/$(VIDEO_ID)/master.m3u8 -o /tmp/demo.m3u8
	@head -10 /tmp/demo.m3u8
	@rm /tmp/demo.m3u8

demo-2: ## Demo: Cache hit performance
	@echo "$(BLUE)Demo 2: Cache hit performance$(NC)"
	@echo "First fetch (cache miss):"
	@curl -s -o /dev/null -w "Time: %{time_total}s\n" http://localhost:8080/hls/$(VIDEO_ID)/segment_0010.m4s
	@echo "Second fetch (cache hit):"
	@curl -s -o /dev/null -w "Time: %{time_total}s\n" http://localhost:8080/hls/$(VIDEO_ID)/segment_0010.m4s

demo-3: ## Demo: Leader election
	@echo "$(BLUE)Demo 3: Leader election$(NC)"
	@make test-election
	@echo "$(YELLOW)Stopping cache-1...$(NC)"
	@docker stop oulu-telco-cdn-oulu-oulu-cache-1
	@sleep 5
	@make test-election
	@echo "$(GREEN)Restarting cache-1...$(NC)"
	@docker start oulu-telco-cdn-oulu-oulu-cache-1

demo-client: ## Run web client against MEC LB
	@echo "$(BLUE)Starting React client$(NC)"
	@cd services/client/frontend && npm install --silent && npm run dev -- --host 0.0.0.0 --port 5173

##@ Cleanup
clean-logs: ## Clean access logs
	@rm -rf data/logs/cache-1/*.ndjson data/logs/cache-2/*.ndjson data/logs/cache-3/*.ndjson data/oulu-logs/*.ndjson
	@echo "$(GREEN)Logs cleaned!$(NC)"

clean: clab-down ## Full cleanup (destroy containerlab topology)

clean-docker: ## Stop and remove all telco containers
	@echo "$(RED)Cleaning up all telco containers...$(NC)"
	@docker ps -a --filter "name=telco" --format "{{.Names}}" | xargs -r docker rm -f
	@docker ps -a --filter "name=oulu-telco" --format "{{.Names}}" | xargs -r docker rm -f
	@echo "$(GREEN)Cleanup complete!$(NC)"

clean-all: clab-down clean-docker ## Nuclear cleanup
	@docker network prune -f
	@echo "$(GREEN)All clean!$(NC)"

.DEFAULT_GOAL := help

# ML Stuff
build-ml: ## Build ML service
	docker build -t telco-cdn-ml:latest -f services/ml-service/Dockerfile services/ml-service

build-fl-client: ## Build FL client
	docker build -t edge-fl-client:latest -f services/edge-fl-client/Dockerfile services/edge-fl-client

build-all-ml: build build-ml build-fl-client ## Build all images

# FL monitoring
fl-status: ## Show FL status
	@echo "$(BLUE)FL Status:$(NC)"
	@curl -s http://localhost:8092/status | jq .

fl-logs: ## Show FL logs
	@docker logs oulu-telco-cdn-oulu-ml-service 2>&1 | tail -20
	@echo ""
	@docker logs oulu-telco-cdn-oulu-oulu-fl-client 2>&1 | tail -20

# Clean logs
clean-logs: ## Clean access logs
	@rm -rf data/oulu-logs/*.ndjson
	@echo "Logs cleaned"
