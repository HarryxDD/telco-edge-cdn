#!/usr/bin/env python3
"""
Generate advanced visualization plots for the evaluation report:
1. Load test performance over time (latency trends from k6)
2. Load distribution across cache nodes (bounded-load hashing)

Usage: python3 scripts/generate-advanced-plots.py <results_dir>
"""

import json
import sys
from pathlib import Path
from datetime import datetime
import numpy as np
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from collections import defaultdict

def parse_k6_results(k6_file):
    """
    Parse k6 NDJSON results file to extract time-series metrics
    Returns: dict with timestamps and corresponding latency values
    """
    print(f"Parsing k6 results from: {k6_file}")
    
    # Data structures for time-series
    http_durations = []  # (timestamp, duration_ms)
    cache_hits_over_time = []
    cache_misses_over_time = []
    vus_over_time = []  # virtual users
    
    start_time = None
    
    with open(k6_file, 'r') as f:
        for line_num, line in enumerate(f, 1):
            if line_num % 100000 == 0:
                print(f"  Processed {line_num} lines...")
            
            try:
                data = json.loads(line.strip())
                
                if data.get('type') == 'Point':
                    metric_name = data.get('metric', '')
                    point_data = data.get('data', {})
                    timestamp_str = point_data.get('time', '')
                    value = point_data.get('value', 0)
                    
                    # Parse timestamp
                    if timestamp_str:
                        try:
                            ts = datetime.fromisoformat(timestamp_str.replace('Z', '+00:00'))
                            if start_time is None:
                                start_time = ts
                            elapsed_seconds = (ts - start_time).total_seconds()
                        except:
                            continue
                    else:
                        continue
                    
                    # Collect relevant metrics
                    if metric_name == 'http_req_duration':
                        http_durations.append((elapsed_seconds, value))
                    elif metric_name == 'cache_hits':
                        cache_hits_over_time.append((elapsed_seconds, value))
                    elif metric_name == 'cache_misses':
                        cache_misses_over_time.append((elapsed_seconds, value))
                    elif metric_name == 'vus':
                        vus_over_time.append((elapsed_seconds, value))
                        
            except json.JSONDecodeError:
                continue
    
    print(f"  Extracted {len(http_durations)} latency data points")
    
    return {
        'http_durations': http_durations,
        'cache_hits': cache_hits_over_time,
        'cache_misses': cache_misses_over_time,
        'vus': vus_over_time
    }

def compute_time_windows(data_points, window_seconds=30):
    """
    Aggregate data points into time windows for cleaner visualization
    Returns: (time_points, p50, p95, p99, mean)
    """
    if not data_points:
        return [], [], [], [], []
    
    # Group by time windows
    windows = defaultdict(list)
    max_time = max(t for t, _ in data_points)
    
    for timestamp, value in data_points:
        window_idx = int(timestamp / window_seconds)
        windows[window_idx].append(value)
    
    # Compute statistics for each window
    time_points = []
    p50_vals = []
    p95_vals = []
    p99_vals = []
    mean_vals = []
    
    for window_idx in sorted(windows.keys()):
        values = windows[window_idx]
        if values:
            time_points.append(window_idx * window_seconds)
            mean_vals.append(np.mean(values))
            p50_vals.append(np.percentile(values, 50))
            p95_vals.append(np.percentile(values, 95))
            p99_vals.append(np.percentile(values, 99))
    
    return time_points, p50_vals, p95_vals, p99_vals, mean_vals

def plot_load_test_performance(results_dir, k6_metrics):
    """
    Generate Figure: Load test performance over time
    Shows latency trends as load ramps from 50 to 200 concurrent users
    """
    print("\nGenerating load test performance plot...")
    
    # Compute windowed statistics for smoother visualization
    time_points, p50, p95, p99, mean = compute_time_windows(
        k6_metrics['http_durations'], 
        window_seconds=30
    )
    
    if not time_points:
        print("  Warning: No time-series data available")
        return
    
    # Convert to minutes for readability
    time_minutes = [t / 60 for t in time_points]
    
    # Create figure with dual y-axes
    fig, ax1 = plt.subplots(figsize=(12, 6))
    
    # Plot latency trends
    ax1.plot(time_minutes, p50, label='P50 (Median)', 
             color='#2ecc71', linewidth=2, marker='o', markersize=4)
    ax1.plot(time_minutes, p95, label='P95', 
             color='#f39c12', linewidth=2, marker='s', markersize=4)
    ax1.plot(time_minutes, p99, label='P99', 
             color='#e74c3c', linewidth=2, marker='^', markersize=4)
    
    ax1.set_xlabel('Test Duration (minutes)', fontsize=12)
    ax1.set_ylabel('Latency (ms)', fontsize=12)
    ax1.set_title('Load Test Performance: Latency Over Time (50→200 Concurrent Users)', 
                  fontsize=14, fontweight='bold')
    ax1.grid(True, alpha=0.3)
    ax1.legend(loc='upper left', fontsize=10)
    
    # Add load ramp annotation
    # Assuming 12-minute test with stages: 2min ramp, 8min steady, 2min ramp down
    ax1.axvspan(0, 2, alpha=0.1, color='blue', label='Ramp Up')
    ax1.axvspan(2, 10, alpha=0.05, color='green')
    ax1.axvspan(10, 12, alpha=0.1, color='blue', label='Ramp Down')
    
    # Add text annotations for load stages
    ax1.text(1, max(p99) * 0.95, '50→200 users', 
             ha='center', va='top', fontsize=9, style='italic')
    ax1.text(6, max(p99) * 0.95, '200 users (steady)', 
             ha='center', va='top', fontsize=9, style='italic')
    
    plt.tight_layout()
    output_file = results_dir / 'load-test-performance.png'
    plt.savefig(output_file, dpi=300, bbox_inches='tight')
    print(f"✓ Created: {output_file}")
    plt.close()
    
    # Print summary statistics
    print(f"  Latency Summary:")
    print(f"    P50: {np.mean(p50):.2f}ms (avg), {min(p50):.2f}-{max(p50):.2f}ms (range)")
    print(f"    P95: {np.mean(p95):.2f}ms (avg), {min(p95):.2f}-{max(p95):.2f}ms (range)")
    print(f"    P99: {np.mean(p99):.2f}ms (avg), {min(p99):.2f}-{max(p99):.2f}ms (range)")

def fetch_load_distribution():
    """
    Attempt to fetch live load distribution from load balancer debug endpoint
    (Only for current active connections, not historical data)
    Returns: dict of node -> active_connections or None
    """
    try:
        import urllib.request
        response = urllib.request.urlopen('http://localhost:8080/debug/ring', timeout=2)
        data = json.loads(response.read())
        
        # Extract request counts per node
        distribution = {}
        for node_status in data:
            node_id = node_status.get('id', 'unknown')
            active_load = node_status.get('active_load', 0)
            distribution[node_id] = active_load
        
        return distribution if distribution else None
    except:
        return None

def extract_actual_load_distribution(results_dir):
    """
    Extract ACTUAL request distribution from access logs
    Returns: dict of node -> request_count or None
    """
    # Try to find access logs
    access_log_paths = [
        Path('data/oulu-logs/access.ndjson'),  # Primary location
        results_dir.parent.parent / 'oulu-logs' / 'access.ndjson',  # Relative
    ]
    
    for log_path in access_log_paths:
        if log_path.exists():
            print(f"  Found access logs: {log_path}")
            try:
                distribution = {}
                with open(log_path, 'r') as f:
                    for line in f:
                        try:
                            entry = json.loads(line.strip())
                            node = entry.get('edge_node_id', 'unknown')
                            distribution[node] = distribution.get(node, 0) + 1
                        except:
                            continue
                
                if distribution and sum(distribution.values()) > 0:
                    total = sum(distribution.values())
                    print(f"  Parsed {total} real requests from access logs")
                    return distribution
            except Exception as e:
                print(f"  Error reading access log: {e}")
                continue
    
    return None

def generate_load_distribution_data():
    """
    DEPRECATED: This function generated dummy comparison data.
    Now we use actual access logs from extract_actual_load_distribution()
    """
    print("Warning: Dummy data function called - should not be used")
    return None

def plot_load_distribution(results_dir):
    """
    Generate Figure: Load distribution across cache nodes
    Uses ACTUAL request distribution from access logs
    """
    print("\nGenerating load distribution plot...")
    
    # Get ACTUAL distribution from access logs
    actual_distribution = extract_actual_load_distribution(results_dir)
    
    if not actual_distribution or sum(actual_distribution.values()) == 0:
        print("  ERROR: No actual distribution data found in access logs")
        print("  Skipping load distribution plot")
        return
    
    # Sort nodes for consistent display
    nodes = sorted(actual_distribution.keys())
    request_counts = [actual_distribution[n] for n in nodes]
    total_requests = sum(request_counts)
    percentages = [(count / total_requests * 100) for count in request_counts]
    
    # Calculate statistics
    avg_count = np.mean(request_counts)
    std_dev = np.std(request_counts)
    max_count = max(request_counts)
    min_count = min(request_counts)
    variance_pct = ((max_count - min_count) / avg_count * 100)
    
    # Create visualization
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(14, 6))
    
    # Left plot: Request counts
    colors = ['#3498db', '#2ecc71', '#9b59b6']
    bars1 = ax1.bar(nodes, request_counts, color=colors, alpha=0.8)
    
    ax1.set_ylabel('Request Count', fontsize=12)
    ax1.set_xlabel('Cache Node', fontsize=12)
    ax1.set_title(f'Actual Request Distribution ({total_requests:,} total requests)', 
                  fontsize=13, fontweight='bold')
    ax1.grid(axis='y', alpha=0.3)
    
    # Add value labels on bars
    for bar, count in zip(bars1, request_counts):
        height = bar.get_height()
        ax1.text(bar.get_x() + bar.get_width()/2., height,
                f'{count:,}\n({count/total_requests*100:.1f}%)',
                ha='center', va='bottom', fontsize=10)
    
    # Add average line
    ax1.axhline(avg_count, color='red', linestyle='--', linewidth=2, 
                alpha=0.7, label=f'Average: {avg_count:.0f}')
    ax1.legend(fontsize=10)
    
    # Right plot: Percentage distribution (pie chart)
    ax2.pie(request_counts, labels=nodes, colors=colors, autopct='%1.1f%%',
            startangle=90, textprops={'fontsize': 11})
    ax2.set_title('Distribution Percentage', fontsize=13, fontweight='bold')
    
    plt.tight_layout()
    output_file = results_dir / 'load-distribution.png'
    plt.savefig(output_file, dpi=300, bbox_inches='tight')
    print(f"✓ Created: {output_file}")
    plt.close()
    
    # Print analysis
    print(f"\n  Real Distribution Analysis:")
    for node in nodes:
        count = actual_distribution[node]
        pct = count / total_requests * 100
        print(f"    {node}: {count:,} requests ({pct:.1f}%)")
    print(f"    Total: {total_requests:,} requests")
    print(f"    Average per node: {avg_count:.0f}")
    print(f"    Std deviation: {std_dev:.0f}")
    print(f"    Load variance: {variance_pct:.1f}%")
    print(f"    Max/Min ratio: {max_count/min_count:.2f}x")

def main():
    if len(sys.argv) < 2:
        results_base = Path('data/evaluation-results')
        if results_base.exists():
            import os
            subdirs = [d for d in results_base.iterdir() if d.is_dir()]
            if subdirs:
                results_dir = max(subdirs, key=os.path.getmtime)
                print(f"Using latest results: {results_dir}")
            else:
                print("No results found")
                sys.exit(1)
        else:
            print("No results directory found")
            sys.exit(1)
    else:
        results_dir = Path(sys.argv[1])
    
    if not results_dir.exists():
        print(f"Error: {results_dir} does not exist")
        sys.exit(1)
    
    print(f"\n{'='*60}")
    print(f"ADVANCED VISUALIZATION GENERATOR")
    print(f"{'='*60}")
    print(f"Results directory: {results_dir}\n")
    
    # Check for k6 results
    k6_file = results_dir / 'k6-results.json'
    if not k6_file.exists():
        print(f"Warning: k6 results not found at {k6_file}")
        print("Skipping load test performance plot\n")
        k6_metrics = None
    else:
        # Parse k6 data
        k6_metrics = parse_k6_results(k6_file)
    
    # Generate visualizations
    print(f"\n{'='*60}")
    print("GENERATING PLOTS")
    print(f"{'='*60}")
    
    if k6_metrics and k6_metrics['http_durations']:
        plot_load_test_performance(results_dir, k6_metrics)
    
    plot_load_distribution(results_dir)
    
    print(f"\n{'='*60}")
    print("✓ COMPLETE")
    print(f"{'='*60}")
    print(f"\nGenerated figures saved to: {results_dir}")
    print("\nAdd these to your LaTeX report:")
    print("  - load-test-performance.png")
    print("  - load-distribution.png")
    print("\n")

if __name__ == '__main__':
    main()
