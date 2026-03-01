#!/usr/bin/env python3
"""
Calculate proper load distribution metrics for bounded-load hashing evaluation
"""

import json
from pathlib import Path
from datetime import datetime
from collections import defaultdict
import numpy as np

def calculate_gini_coefficient(distribution):
    """
    Calculate Gini coefficient (0 = perfect equality, 1 = total inequality)
    """
    values = sorted(distribution)
    n = len(values)
    cumsum = 0
    for i, val in enumerate(values):
        cumsum += (2 * (i + 1) - n - 1) * val
    return cumsum / (n * sum(values))

def analyze_load_distribution(access_log_path):
    """
    Analyze load distribution with proper time-based metrics
    """
    print(f"Analyzing: {access_log_path}\n")
    
    # Parse all requests
    requests_by_node = defaultdict(list)
    node_counts = defaultdict(int)
    
    with open(access_log_path, 'r') as f:
        for line in f:
            try:
                entry = json.loads(line.strip())
                node = entry.get('edge_node_id')
                timestamp_str = entry.get('timestamp')
                
                if node and timestamp_str:
                    ts = datetime.fromisoformat(timestamp_str.replace('Z', '+00:00'))
                    requests_by_node[node].append(ts)
                    node_counts[node] += 1
            except:
                continue
    
    nodes = sorted(node_counts.keys())
    counts = [node_counts[n] for n in nodes]
    total_requests = sum(counts)
    
    # Calculate test duration
    all_timestamps = []
    for timestamps in requests_by_node.values():
        all_timestamps.extend(timestamps)
    all_timestamps.sort()
    
    start_time = all_timestamps[0]
    end_time = all_timestamps[-1]
    duration_seconds = (end_time - start_time).total_seconds()
    duration_minutes = duration_seconds / 60
    
    print("="*60)
    print("LOAD DISTRIBUTION METRICS")
    print("="*60)
    print(f"\nTest Duration: {duration_minutes:.1f} minutes ({duration_seconds:.0f} seconds)")
    print(f"Total Requests: {total_requests:,}")
    print()
    
    # 1. Request counts and percentages
    print("1. REQUEST DISTRIBUTION:")
    for node in nodes:
        count = node_counts[node]
        pct = count / total_requests * 100
        print(f"   {node}: {count:,} requests ({pct:.1f}%)")
    print()
    
    # 2. Throughput (requests per second)
    print("2. SUSTAINED THROUGHPUT (req/s):")
    throughputs = []
    for node in nodes:
        count = node_counts[node]
        throughput = count / duration_seconds
        throughputs.append(throughput)
        pct = count / total_requests * 100
        print(f"   {node}: {throughput:.2f} req/s ({pct:.1f}% of total)")
    
    total_throughput = sum(throughputs)
    print(f"   Total:  {total_throughput:.2f} req/s")
    print()
    
    # 3. Statistical metrics
    mean_count = np.mean(counts)
    std_dev = np.std(counts)
    cv = std_dev / mean_count if mean_count > 0 else 0
    gini = calculate_gini_coefficient(counts)
    
    print("3. LOAD BALANCE METRICS:")
    print(f"   Mean requests/node: {mean_count:.0f}")
    print(f"   Std deviation: {std_dev:.0f}")
    print(f"   Coefficient of Variation (CV): {cv:.3f}")
    print(f"     → {cv*100:.1f}% variation from mean")
    print(f"     → Target: < 0.15 for good balance")
    print()
    
    print(f"   Gini Coefficient: {gini:.3f}")
    print(f"     → 0.00 = perfect equality")
    print(f"     → 1.00 = total inequality")
    print(f"     → Target: < 0.10 for good balance")
    print()
    
    # 4. Imbalance ratios
    max_count = max(counts)
    min_count = min(counts)
    max_node = nodes[counts.index(max_count)]
    min_node = nodes[counts.index(min_count)]
    
    print("4. IMBALANCE RATIOS:")
    print(f"   Max/Min ratio: {max_count/min_count:.2f}×")
    print(f"     → {max_node}: {max_count:,} / {min_node}: {min_count:,}")
    print(f"     → Target: < 1.5× for good balance")
    print()
    
    max_min_deviation = (max_count - mean_count) / mean_count * 100
    print(f"   Max deviation from mean: {max_min_deviation:+.1f}%")
    print(f"   Min deviation from mean: {(min_count - mean_count) / mean_count * 100:+.1f}%")
    print()
    
    # 5. Bounded-load effectiveness
    avg_throughput = np.mean(throughputs)
    max_throughput = max(throughputs)
    threshold_125 = avg_throughput * 1.25
    
    print("5. BOUNDED-LOAD ANALYSIS:")
    print(f"   Average throughput: {avg_throughput:.2f} req/s")
    print(f"   Max observed: {max_throughput:.2f} req/s")
    print(f"   125% threshold: {threshold_125:.2f} req/s")
    
    if max_throughput > threshold_125:
        exceed_pct = ((max_throughput/threshold_125 - 1)*100)
        print(f"   ⚠ Max exceeds threshold by {exceed_pct:.1f}%")
        
        if exceed_pct > 15:
            print(f"   → Bounded-load likely not working effectively")
            print(f"   → Check implementation or increase concurrency")
        else:
            print(f"   → Bounded-load triggered but load still slightly imbalanced")
            print(f"   → This can happen with Zipf workloads (hot segments)")
    else:
        print(f"   ✓ Max within threshold")
        print(f"   → Bounded-load not triggered (not needed)")
    print()
    
    # 6. Interpretation
    print("="*60)
    print("INTERPRETATION:")
    print("="*60)
    
    if cv < 0.15:
        print("✓ EXCELLENT: Load well distributed (CV < 15%)")
    elif cv < 0.30:
        print("○ ACCEPTABLE: Moderate imbalance (CV 15-30%)")
    else:
        print("✗ POOR: Significant imbalance (CV > 30%)")
    
    if gini < 0.10:
        print("✓ EXCELLENT: Near-perfect equality (Gini < 0.10)")
    elif gini < 0.25:
        print("○ ACCEPTABLE: Moderate inequality (Gini 0.10-0.25)")
    else:
        print("✗ POOR: High inequality (Gini > 0.25)")
    
    print()
    print("RECOMMENDATION:")
    if cv > 0.50 or gini > 0.30:
        print("  ✗ CRITICAL: Load distribution is very poor.")
        print("  Bounded-load hashing is NOT working at all.")
        print()
        print("  Root cause: Implementation tracks instantaneous connections instead")
        print("              of time-windowed request rates.")
        print()
        print("  Solutions:")
        print("  1. Track time-windowed request rate (10s windows)")
        print("  2. Use sliding windows or EWMA for smoother rate calculation")
        print("  3. Add rerouting logs to verify bounded-load triggers")
    elif cv > 0.30 or gini > 0.20:
        print("  ⚠ MODERATE: Bounded-load working but not perfectly.")
        print()
        print("  Likely causes:")
        print("  1. Zipf distribution: Popular segments naturally hash to same node")
        print("  2. Low concurrency: Not enough traffic to trigger frequent rerouting")
        print("  3. Consistent hashing: Same keys always prefer same nodes")
        print()
        print("  Potential improvements:")
        print("  1. Add virtual nodes (100+ per physical node) to spread hash targets")
        print("  2. Use power-of-two-choices (check 2 positions, pick best)")
        print("  3. Test with higher load (50+ req/s) to see stronger effect")
        print("  4. Consider segment-aware routing for top-10% popular segments")
    else:
        print("  ✓ EXCELLENT: Load distribution is acceptable.")
        print("  Bounded-load hashing is working effectively.")
    
    print("="*60)

if __name__ == '__main__':
    log_path = Path('data/oulu-logs/access.ndjson')
    if log_path.exists():
        analyze_load_distribution(log_path)
    else:
        print(f"Error: {log_path} not found")
