#!/bin/bash
# Pre-Demo Setup Script
# Run this 10 minutes before your demo to ensure everything is ready

set -e

echo " CDN Demo Setup Script"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Step 1/5: Cleaning previous state...${NC}"
make clean > /dev/null 2>&1 || true

echo -e "${BLUE}Step 2/5: Building services (this may take 2-3 minutes)...${NC}"
make build

echo -e "${BLUE}Step 3/5: Starting cluster...${NC}"
make up > /dev/null 2>&1

echo -e "${YELLOW}Step 4/5: Waiting for cluster to stabilize (15 seconds)...${NC}"
sleep 15

echo -e "${BLUE}Step 5/5: Verifying cluster health...${NC}"
echo ""

# Check each service
check_service() {
    local name=$1
    local url=$2
    
    if curl -s -f "$url" > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} $name is healthy"
        return 0
    else
        echo -e "  ${RED}✗${NC} $name is NOT responding"
        return 1
    fi
}

all_healthy=true

check_service "Load Balancer" "http://localhost:8090/health" || all_healthy=false
check_service "Cache Node 1" "http://localhost:8081/health" || all_healthy=false
check_service "Cache Node 2" "http://localhost:8082/health" || all_healthy=false
check_service "Cache Node 3" "http://localhost:8083/health" || all_healthy=false
check_service "Origin Server" "http://localhost:8443/health" || all_healthy=false

echo ""

if [ "$all_healthy" = true ]; then
    echo -e "${GREEN} ALL SYSTEMS READY FOR DEMO!${NC}"
    echo ""
    echo -e "${YELLOW} Important URLs:${NC}"
    echo "   Frontend:       http://localhost:3000"
    echo "   Load Balancer:  http://localhost:8090"
    echo "   Cache Node 1:   http://localhost:8081"
    echo "   Cache Node 2:   http://localhost:8082"
    echo "   Cache Node 3:   http://localhost:8083"
    echo "   Origin Server:  http://localhost:8443"
    echo ""
    echo -e "${YELLOW} Quick Start Commands:${NC}"
    echo "   make status              # Check cluster health"
    echo "   make demo                # Run automated demo"
    echo "   make test-all            # Run all tests"
    echo "   make logs-cache-all      # Watch live logs"
    echo ""
    echo -e "${YELLOW} Full Demo Guide:${NC}"
    echo "   docs/demo.md"
else
    echo -e "${RED} SOME SERVICES ARE NOT HEALTHY${NC}"
    echo ""
    echo -e "${YELLOW}Troubleshooting:${NC}"
    echo "   make logs           # Check all logs"
    echo "   make down           # Stop everything"
    echo "   make rebuild        # Try rebuilding"
    echo ""
    echo "Common issues:"
    echo "   - Ports already in use (stop other Docker containers)"
    echo "   - Docker not running (start Docker Desktop)"
    echo "   - Insufficient resources (increase Docker memory limit)"
    exit 1
fi
