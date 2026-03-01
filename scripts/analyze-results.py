#!/usr/bin/env python3
"""
Analyze evaluation results and generate visualizations for report
Usage: python3 scripts/analyze-results.py <results_dir>
"""

import json
import sys
import os
import numpy as np
import matplotlib
matplotlib.use('Agg')  # Non-interactive backend
import matplotlib.pyplot as plt
from pathlib import Path

def load_json(filepath):
    """Load JSON file"""
    try:
        with open(filepath) as f:
            return json.load(f)
    except Exception as e:
        print(f"Warning: Could not load {filepath}: {e}")
        return None

def plot_baseline_comparison(results_dir):
    """Generate baseline cold vs warm comparison"""
    baseline_file = results_dir / 'baseline' / 'baseline-stats.json'
    if not baseline_file.exists():
        print(f"Skipping baseline plot: {baseline_file} not found")
        return
    
    data = load_json(baseline_file)
    if not data:
        return
    
    # Extract data
    categories = ['First Request\n(Cold)', 'Warm Cache', 'Direct Origin']
    means = [
        data['first_request']['mean'],
        data['warm_cache']['mean'],
        data['direct_origin']['mean']
    ]
    p95s = [
        data['first_request']['p95'],
        data['warm_cache']['p95'],
        data['direct_origin']['p95']
    ]
    
    # Create figure with two subplots
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(12, 5))
    
    # Mean comparison
    bars1 = ax1.bar(categories, means, color=['#e74c3c', '#2ecc71', '#3498db'])
    ax1.set_ylabel('Latency (ms)')
    ax1.set_title('Mean Latency Comparison')
    ax1.grid(axis='y', alpha=0.3)
    
    # Add value labels on bars
    for bar in bars1:
        height = bar.get_height()
        ax1.text(bar.get_x() + bar.get_width()/2., height,
                f'{height:.1f}ms', ha='center', va='bottom')
    
    # P95 comparison
    bars2 = ax2.bar(categories, p95s, color=['#e74c3c', '#2ecc71', '#3498db'])
    ax2.set_ylabel('Latency (ms)')
    ax2.set_title('P95 Latency Comparison')
    ax2.grid(axis='y', alpha=0.3)
    
    for bar in bars2:
        height = bar.get_height()
        ax2.text(bar.get_x() + bar.get_width()/2., height,
                f'{height:.1f}ms', ha='center', va='bottom')
    
    plt.tight_layout()
    output_file = results_dir / 'baseline-comparison.png'
    plt.savefig(output_file, dpi=300, bbox_inches='tight')
    print(f"✓ Created: {output_file}")
    plt.close()

def plot_cache_hit_distribution(results_dir):
    """Generate cache hit ratio visualizations"""
    cache_file = results_dir / 'cache-hit' / 'analysis.json'
    if not cache_file.exists():
        print(f"Skipping cache hit plot: {cache_file} not found")
        return
    
    data = load_json(cache_file)
    if not data:
        return
    
    # Create figure with two subplots
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(14, 5))
    
    # Pie chart: Hit vs Miss ratio
    hits = data['cache_hits']
    misses = data['cache_misses']
    sizes = [hits, misses]
    labels = [f'Hits ({hits})', f'Misses ({misses})']
    colors = ['#2ecc71', '#e74c3c']
    explode = (0.1, 0)
    
    ax1.pie(sizes, explode=explode, labels=labels, colors=colors,
            autopct='%1.1f%%', shadow=True, startangle=90)
    ax1.set_title(f'Cache Hit Ratio: {data["hit_ratio_percent"]}%')
    
    # Bar chart: Distribution by segment
    segments = sorted(data['segment_stats'].items(), 
                     key=lambda x: x[1]['count'], reverse=True)
    segment_names = [s[0].replace('segment_', 'S') for s in segments[:8]]
    segment_counts = [s[1]['count'] for s in segments[:8]]
    segment_percents = [s[1]['percent'] for s in segments[:8]]
    
    bars = ax2.bar(segment_names, segment_percents, color='#3498db')
    ax2.set_xlabel('Segment')
    ax2.set_ylabel('Request Percentage (%)')
    ax2.set_title('Request Distribution (Zipf Pattern)')
    ax2.grid(axis='y', alpha=0.3)
    
    # Add labels
    for bar, count in zip(bars, segment_counts):
        height = bar.get_height()
        ax2.text(bar.get_x() + bar.get_width()/2., height,
                f'{count}', ha='center', va='bottom', fontsize=8)
    
    plt.tight_layout()
    output_file = results_dir / 'cache-hit-analysis.png'
    plt.savefig(output_file, dpi=300, bbox_inches='tight')
    print(f"✓ Created: {output_file}")
    plt.close()

def plot_latency_distribution(results_dir):
    """Plot latency distribution from baseline test"""
    warm_file = results_dir / 'baseline' / 'warm-cache-latency.txt'
    origin_file = results_dir / 'baseline' / 'origin-latency.txt'
    
    if not warm_file.exists() or not origin_file.exists():
        print("Skipping latency distribution: files not found")
        return
    
    # Load data
    with open(warm_file) as f:
        warm_data = [float(line.strip()) for line in f if line.strip()]
    
    with open(origin_file) as f:
        origin_data = [float(line.strip()) for line in f if line.strip()]
    
    # Create histogram
    fig, ax = plt.subplots(figsize=(10, 6))
    
    bins = np.linspace(0, max(max(warm_data), max(origin_data)), 50)
    ax.hist(warm_data, bins=bins, alpha=0.7, label='Warm Cache', color='#2ecc71')
    ax.hist(origin_data, bins=bins, alpha=0.7, label='Direct Origin', color='#e74c3c')
    
    # Add percentile lines
    ax.axvline(np.percentile(warm_data, 95), color='#27ae60', 
               linestyle='--', label=f'Warm P95: {np.percentile(warm_data, 95):.1f}ms')
    ax.axvline(np.percentile(origin_data, 95), color='#c0392b', 
               linestyle='--', label=f'Origin P95: {np.percentile(origin_data, 95):.1f}ms')
    
    ax.set_xlabel('Latency (ms)')
    ax.set_ylabel('Frequency')
    ax.set_title('Latency Distribution: Cache vs Origin')
    ax.legend()
    ax.grid(alpha=0.3)
    
    output_file = results_dir / 'latency-distribution.png'
    plt.savefig(output_file, dpi=300, bbox_inches='tight')
    print(f"✓ Created: {output_file}")
    plt.close()

def generate_summary_table(results_dir):
    """Generate summary table of all metrics"""
    summary = {}
    
    # Baseline metrics
    baseline_file = results_dir / 'baseline' / 'baseline-stats.json'
    if baseline_file.exists():
        data = load_json(baseline_file)
        if data:
            summary['baseline'] = {
                'Cold Cache P95 (ms)': f"{data['first_request']['p95']:.2f}",
                'Warm Cache P95 (ms)': f"{data['warm_cache']['p95']:.2f}",
                'Speedup Factor': f"{data['first_request']['mean'] / data['warm_cache']['mean']:.1f}x"
            }
    
    # Cache hit ratio
    cache_file = results_dir / 'cache-hit' / 'analysis.json'
    if cache_file.exists():
        data = load_json(cache_file)
        if data:
            summary['cache'] = {
                'Hit Ratio': f"{data['hit_ratio_percent']:.1f}%",
                'Total Requests': data['total_requests'],
                'Cache Hits': data['cache_hits']
            }
    
    # Leader election
    election_file = results_dir / 'election' / 'election-summary.txt'
    if election_file.exists():
        with open(election_file) as f:
            lines = f.readlines()
            summary['election'] = {line.split(': ')[0]: line.split(': ')[1].strip() 
                                  for line in lines if ': ' in line}
    
    # Save summary
    output_file = results_dir / 'summary.json'
    with open(output_file, 'w') as f:
        json.dump(summary, f, indent=2)
    
    print(f"✓ Created: {output_file}")
    
    # Print to console
    print("\n" + "=" * 60)
    print("EVALUATION SUMMARY")
    print("=" * 60)
    for section, metrics in summary.items():
        print(f"\n{section.upper()}:")
        for key, value in metrics.items():
            print(f"  {key:25s}: {value}")
    print("=" * 60 + "\n")

def main():
    if len(sys.argv) < 2:
        results_base = Path('data/evaluation-results')
        # Find latest results directory
        if results_base.exists():
            subdirs = [d for d in results_base.iterdir() if d.is_dir()]
            if subdirs:
                results_dir = max(subdirs, key=os.path.getmtime)
                print(f"Using latest results: {results_dir}")
            else:
                print("No results found in data/evaluation-results/")
                print("Usage: python3 scripts/analyze-results.py <results_dir>")
                sys.exit(1)
        else:
            print("No results directory found")
            sys.exit(1)
    else:
        results_dir = Path(sys.argv[1])
    
    if not results_dir.exists():
        print(f"Error: {results_dir} does not exist")
        sys.exit(1)
    
    print(f"\nAnalyzing results from: {results_dir}\n")
    
    # Generate visualizations
    print("Generating visualizations...")
    plot_baseline_comparison(results_dir)
    plot_cache_hit_distribution(results_dir)
    plot_latency_distribution(results_dir)
    
    # Generate summary
    print("\nGenerating summary...")
    generate_summary_table(results_dir)
    
    print("\n✓ Analysis complete!")
    print(f"Results saved to: {results_dir}")

if __name__ == '__main__':
    main()
