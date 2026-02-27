// k6 Load Test for Telco-Edge CDN
// Usage: k6 run --out json=results.json k6-test.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend, Rate } from 'k6/metrics';

// Custom metrics
const cacheHits = new Counter('cache_hits');
const cacheMisses = new Counter('cache_misses');
const requestDuration = new Trend('request_duration_ms');
const errorRate = new Rate('error_rate');

// Test configuration
export const options = {
  stages: [
    { duration: '1m', target: 50 },
    { duration: '3m', target: 50 },
    { duration: '1m', target: 100 },
    { duration: '3m', target: 100 },
    { duration: '1m', target: 200 },
    { duration: '2m', target: 200 },
    { duration: '1m', target: 0 },
  ],
  thresholds: {
    'http_req_duration': ['p(95)<100'],     // 95% of requests < 100ms
    'http_req_failed': ['rate<0.01'],        // <1% errors
    'error_rate': ['rate<0.01'],
  },
};

// Video segments to test (simulating different content)
const segments = [
  'segment_0000.m4s',
  'segment_0001.m4s',
  'segment_0002.m4s',
  'segment_0003.m4s',
  'segment_0004.m4s',
  'segment_0005.m4s',
  'segment_0006.m4s',
  'segment_0007.m4s',
  'segment_0008.m4s',
  'segment_0009.m4s',
];

const videoId = 'wolf-1770316220';
const baseUrl = __ENV.BASE_URL || 'http://localhost:8080';

export default function() {
  // Zipf-like distribution: some segments more popular than others
  let segment;
  const rand = Math.random() * 100;
  
  if (rand < 40) {
    // 40% of requests go to segment 0 (most popular)
    segment = segments[0];
  } else if (rand < 65) {
    // 25% to segment 1
    segment = segments[1];
  } else if (rand < 80) {
    // 15% to segment 2
    segment = segments[2];
  } else if (rand < 90) {
    // 10% to segment 3
    segment = segments[3];
  } else {
    // 10% distributed among remaining segments
    segment = segments[4 + Math.floor(Math.random() * 6)];
  }

  const url = `${baseUrl}/hls/${videoId}/${segment}`;
  
  const startTime = Date.now();
  const response = http.get(url, {
    tags: { name: segment },
  });
  const duration = Date.now() - startTime;

  // Check response
  const success = check(response, {
    'status is 200': (r) => r.status === 200,
    'response time < 100ms': (r) => r.timings.duration < 100,
  });

  // Record metrics
  requestDuration.add(duration);
  
  if (!success) {
    errorRate.add(1);
  } else {
    errorRate.add(0);
  }

  // Track cache hits/misses from X-Cache header
  if (response.headers['X-Cache'] === 'HIT') {
    cacheHits.add(1);
  } else if (response.headers['X-Cache'] === 'MISS') {
    cacheMisses.add(1);
  }

  // Small sleep to simulate realistic user behavior
  sleep(Math.random() * 2 + 0.5); // 0.5-2.5 seconds
}

export function handleSummary(data) {
  return {
    'summary.json': JSON.stringify(data, null, 2),
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
  };
}

function textSummary(data, options = {}) {
  const indent = options.indent || '';
  const enableColors = options.enableColors || false;
  
  let summary = '\n\n';
  summary += `${indent} Test Summary\n\n`;
  
  // Requests
  const requests = data.metrics.http_reqs;
  if (requests) {
    summary += `${indent}Total Requests: ${requests.values.count}\n`;
    summary += `${indent}Request Rate: ${requests.values.rate.toFixed(2)}/s\n\n`;
  }
  
  // Duration
  const duration = data.metrics.http_req_duration;
  if (duration) {
    summary += `${indent}Request Duration:\n`;
    summary += `${indent}  avg: ${duration.values.avg.toFixed(2)}ms\n`;
    summary += `${indent}  med: ${duration.values.med.toFixed(2)}ms\n`;
    summary += `${indent}  p95: ${duration.values['p(95)'].toFixed(2)}ms\n`;
    summary += `${indent}  p99: ${duration.values['p(99)'].toFixed(2)}ms\n\n`;
  }
  
  // Cache metrics (if available)
  if (data.metrics.cache_hits && data.metrics.cache_misses) {
    const hits = data.metrics.cache_hits.values.count;
    const misses = data.metrics.cache_misses.values.count;
    const total = hits + misses;
    const hitRatio = total > 0 ? (hits / total * 100).toFixed(2) : 0;
    
    summary += `${indent}Cache Performance:\n`;
    summary += `${indent}  Hits: ${hits}\n`;
    summary += `${indent}  Misses: ${misses}\n`;
    summary += `${indent}  Hit Ratio: ${hitRatio}%\n\n`;
  }
  
  // Errors
  const failed = data.metrics.http_req_failed;
  if (failed) {
    const failRate = (failed.values.rate * 100).toFixed(2);
    summary += `${indent}Error Rate: ${failRate}%\n\n`;
  }
  
  summary += `${indent}\n\n`;
  
  return summary;
}
