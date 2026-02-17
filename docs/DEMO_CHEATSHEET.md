# DEMO CHEAT SHEET - Quick Reference

## Pre-Demo (10 min before)
```bash
./scripts/demo-setup.sh
# OR
make demo-setup
```

---

## Demo Flow (30 min)

### Architecture & Health (5 min)
```bash
make status                          # Show cluster health
curl http://localhost:8081/coordination/status | jq  # Check leader
curl http://localhost:8082/coordination/status | jq
curl http://localhost:8083/coordination/status | jq
```
**Show:** 3 cache nodes + origin + load balancer, leader elected

---

### Video Upload & Caching (7 min)
```bash
# Open browser: http://localhost:3000
# Click "Upload Video" → Select file → Watch progress

# First fetch (MISS - slow)
curl -w "\nTime: %{time_total}s\n" http://localhost:8090/hls/wolf-1770316220/segment_0000.m4s -o /dev/null -s

# Second fetch (HIT - fast!)
curl -w "\nTime: %{time_total}s\n" http://localhost:8090/hls/wolf-1770316220/segment_0000.m4s -o /dev/null -s

# View logs
make logs-cache-1 | grep "CACHE HIT\|CACHE MISS"
```
**Show:** Cache hit is 5-10x faster than miss

---

### Cache Stampede Prevention (5 min)
```bash
make stampede-test                   # 20 concurrent requests

# Verify lock coordination
make logs-cache-locks                # See "Got lock" (only 1 node!)

# Verify origin savings
make logs-origin-fetch               # Only 1-2 origin fetches!
```
**Show:** 20 requests → only 1 origin fetch (19 saved!)

---

### Gossip Protocol (5 min)
```bash
make logs-cache-gossip               # Watch nodes share inventory

# Fetch on cache-1
curl http://localhost:8081/hls/wolf-1770316220/segment_0005.m4s -o /dev/null -s

# Wait for gossip
sleep 5

# cache-2 knows about it now (peer fetch!)
curl http://localhost:8082/hls/wolf-1770316220/segment_0005.m4s -o /dev/null -s
```
**Show:** Nodes share cache state, fetch from peers

---

### Load Balancing (3 min)
```bash
make test-load                       # 50 requests with 10 concurrent

# Check distribution
echo "Cache-1:" && docker logs telco-cache-1 2>&1 | grep "GET /hls" | wc -l
echo "Cache-2:" && docker logs telco-cache-2 2>&1 | grep "GET /hls" | wc -l
echo "Cache-3:" && docker logs telco-cache-3 2>&1 | grep "GET /hls" | wc -l
```
**Show:** Requests evenly distributed (consistent hashing)

---

### Node Failure & Recovery (5 min)
```bash
make kill-leader                     # Kill cache-3

# Watch re-election in logs (Terminal 2)
make logs-cache-all | grep "election\|leader"

# System still works!
curl -w "Time: %{time_total}s\n" http://localhost:8090/hls/wolf-1770316220/segment_0010.m4s -o /dev/null -s

make status                          # 2/3 nodes healthy

make recover-all                     # Restart node
make status                          # All healthy again
```
**Show:** Automatic re-election, system stays online

---

### Origin Latency Simulation (5 min)
```bash
# Baseline (cached - fast)
curl -w "\nTime: %{time_total}s\n" http://localhost:8090/hls/wolf-1770316220/segment_0001.m4s -o /dev/null -s

# Inject 2s latency to origin
make latency-demo

# Cold segment (origin fetch - SLOW!)
curl -w "\nTime: %{time_total}s\n" http://localhost:8090/hls/wolf-1770316220/segment_0020.m4s -o /dev/null -s

# Same segment again (cached - FAST!)
curl -w "\nTime: %{time_total}s\n" http://localhost:8090/hls/wolf-1770316220/segment_0020.m4s -o /dev/null -s

# OR run automated test
make latency-demo-test

# Clean up
make latency-remove
```
**Show:** 2+ seconds vs 0.05 seconds (40x faster!)

---

### Integration Test (3 min)
```bash
make test-all                        # All automated tests

# Play video in browser
# http://localhost:3000 → Select video → Play
# Open F12 Network tab to show segment loading
```
**Show:** End-to-end working system

---

## Key Talking Points

1. **Bully Election** - Highest ID becomes leader, automatic failover
2. **Lock Coordination** - Prevents cache stampede (Thunder Herd)
3. **Gossip Protocol** - Distributed knowledge, peer fetching
4. **Consistent Hashing** - Load distribution, cache locality
5. **Fault Tolerance** - Works with 2/3 nodes, auto-heals
6. **CDN Benefits** - 10-100x faster cached responses

---

## Key Metrics

```bash
# Cache efficiency
docker logs telco-cache-1 2>&1 | grep "CACHE HIT" | wc -l    # Should be HIGH
docker logs telco-cache-1 2>&1 | grep "CACHE MISS" | wc -l   # Should be LOW

# Origin savings (stampede test)
make logs-origin-fetch | wc -l    # Should be ~1-2 for 20 requests

# Gossip activity
make logs-cache-gossip | wc -l    # Periodic messages

# Election events
docker logs telco-cache-1 2>&1 | grep "election" | wc -l
```

---

## Emergency Commands

```bash
make down           # Stop everything
make rebuild        # Nuclear option
make logs          # Debug issues
make ps            # Container status
```

---
