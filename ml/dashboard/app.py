from flask import Flask, jsonify, render_template_string
import pandas as pd
import numpy as np
import os
import pickle
import time
import threading
import requests
from pathlib import Path
from collections import deque, defaultdict

app = Flask(__name__)

DATA_PATH   = os.environ.get('LOG_PATH',
    r"D:\1-Oulu-Courses\Distributed-System\Data\edge_logs.ndjson")
MODEL_PATH  = os.environ.get('MODEL_PATH',
    r"D:\1-Oulu-Courses\Distributed-System\Data\models\best_xgb_model.pkl")
SCALER_PATH = os.environ.get('SCALER_PATH',
    r"D:\1-Oulu-Courses\Distributed-System\Data\models\scaler.pkl")
AGGREGATOR  = os.environ.get('AGGREGATOR_URL', "http://127.0.0.1:8092")

print("Loading model...")
with open(MODEL_PATH,  'rb') as f: model  = pickle.load(f)
with open(SCALER_PATH, 'rb') as f: scaler = pickle.load(f)

print("Loading logs...")
df = pd.read_json(DATA_PATH, lines=True)
df['timestamp'] = pd.to_datetime(df['timestamp'])
df = df.sort_values('timestamp').reset_index(drop=True)
LOGS = df.to_dict('records')

state = {
    'total_requests' : 0,
    'cache_hits'     : 0,
    'cache_misses'   : 0,
    'recent_requests': deque(maxlen=60),
    'node_stats'     : defaultdict(lambda: {'hits':0,'misses':0,'requests':0}),
    'video_counts'   : defaultdict(int),
    'spike_alerts'   : [],
    'request_rate'   : deque(maxlen=20),
    'fl'             : {
        'round'     : 0,
        'nodes'     : [],
        'avg_f1'    : 0,
        'avg_auc'   : 0,
        'history'   : [],
        'status'    : 'waiting',
        'reachable' : False,
    }
}
state_lock = threading.Lock()


def replay_logs():
    idx = 0
    per_second_count = 0
    last_tick = time.time()

    while True:
        log = LOGS[idx % len(LOGS)]
        idx += 1

        with state_lock:
            state['total_requests'] += 1
            per_second_count        += 1

            if log['cache_hit']:
                state['cache_hits']   += 1
            else:
                state['cache_misses'] += 1

            node = log['edge_node_id']
            state['node_stats'][node]['requests'] += 1
            if log['cache_hit']:
                state['node_stats'][node]['hits']   += 1
            else:
                state['node_stats'][node]['misses'] += 1

            state['video_counts'][log['video_id']] += 1
            state['recent_requests'].append({
                'time'    : str(log['timestamp'])[-15:-7],
                'video'   : log['video_id'],
                'node'    : log['edge_node_id'],
                'hit'     : bool(log['cache_hit']),
                'response': int(log['response_time_ms']),
                'protocol': str(log['protocol']),
              })

        if time.time() - last_tick >= 1.0:
            with state_lock:
                state['request_rate'].append(per_second_count)
                per_second_count = 0

                #ML spike predictions
                alerts = []
                top_videos = sorted(
                    state['video_counts'].items(),
                    key=lambda x: x[1], reverse=True
                )[:6]

                for vid, count in top_videos:
                    features = np.array([[
                        time.localtime().tm_hour,
                        time.localtime().tm_wday,
                        1 if time.localtime().tm_wday >= 5 else 0,
                        1 if 18 <= time.localtime().tm_hour <= 23 else 0,
                        float(count),
                        float(count) * 0.8,
                        float(count) * 0.6,
                        float(count) / (float(count)*0.6 + 1e-6),
                        state['cache_hits'] / max(state['total_requests'], 1),
                        150.0, 0.05, 2875.0, float(count),
                    ]])
                    try:
                        prob = model.predict_proba(scaler.transform(features))[0][1]
                        alerts.append({
                            'video'   : vid,
                            'prob'    : round(float(prob) * 100, 1),
                            'alert'   : bool(prob >= 0.3),
                            'requests': int(count),
                        })
                    except:
                        pass
                state['spike_alerts'] = sorted(
                    alerts, key=lambda x: x['prob'], reverse=True)

            last_tick = time.time()

        time.sleep(0.05)


def poll_aggregator():
    while True:
        try:
            r = requests.get(f"{AGGREGATOR}/status", timeout=3)
            if r.status_code == 200:
                data = r.json()
                with state_lock:
                    history = data.get('round_history', [])
                    latest  = history[-1] if history else {}
                    state['fl'] = {
                        'round'    : data.get('fl_round', 0),
                        'nodes'    : data.get('participating_nodes', []),
                        'avg_f1'   : latest.get('avg_f1',  0),
                        'avg_auc'  : latest.get('avg_auc', 0),
                        'history'  : history[-5:],
                        'status'   : 'active' if data.get('fl_round', 0) > 0 else 'waiting',
                        'reachable': True,
                        'best_node': latest.get('best_node', 'N/A'),
                        'n_nodes'  : latest.get('n_nodes', 0),
                    }
        except:
            with state_lock:
                state['fl']['reachable'] = False
                state['fl']['status']    = 'aggregator offline'

        time.sleep(3)   


threading.Thread(target=replay_logs,    daemon=True).start()
threading.Thread(target=poll_aggregator, daemon=True).start()


@app.route('/api/stats')
def api_stats():
    with state_lock:
        total = state['total_requests']
        hr    = round(state['cache_hits'] / max(total,1) * 100, 1)
        nodes = {
            node: {
                'requests': s['requests'],
                'hit_rate': round(s['hits'] / max(s['requests'],1) * 100, 1)
            }
            for node, s in state['node_stats'].items()
        }
        return jsonify({
            'total_requests': total,
            'cache_hit_rate': hr,
            'cache_hits'    : state['cache_hits'],
            'cache_misses'  : state['cache_misses'],
            'request_rate'  : list(state['request_rate']),
            'node_stats'    : nodes,
            'spike_alerts'  : state['spike_alerts'],
            'recent'        : list(state['recent_requests'])[-8:],
            'top_videos'    : sorted(
                                  state['video_counts'].items(),
                                  key=lambda x: x[1], reverse=True
                              )[:8],
            'fl'            : state['fl'],
        })


HTML = """
<!DOCTYPE html>
<html>
<head>
<title>Telco-Edge CDN - ML Dashboard</title>
<meta charset="utf-8">
<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
<style>
  * { box-sizing:border-box; margin:0; padding:0; }
  body { background:#0f1117; color:#e0e0e0;
         font-family:'Segoe UI',sans-serif; }

  /* ── Header ── */
  .header {
    background:linear-gradient(135deg,#1a1f2e,#2d3561);
    padding:16px 28px;
    display:flex; align-items:center; justify-content:space-between;
    border-bottom:1px solid #2a2f45;
  }
  .header h1 { font-size:1.2rem; color:#7eb8f7; font-weight:600; }
  .header-right { display:flex; gap:20px; align-items:center; }
  .live-badge { display:flex; align-items:center; gap:6px;
                font-size:0.8rem; color:#aaa; }
  .dot { width:9px; height:9px; border-radius:50%; background:#4caf50;
         animation:pulse 1.5s infinite; }
  .dot.offline { background:#f44336; animation:none; }
  @keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.3} }

  /* ── Section titles ── */
  .section-title {
    padding:16px 28px 8px;
    font-size:0.75rem; text-transform:uppercase;
    letter-spacing:1px; color:#555;
    border-top:1px solid #1e2235;
  }

  /* ── KPI grid ── */
  .kpi-grid { display:grid; grid-template-columns:repeat(4,1fr);
              gap:14px; padding:14px 28px; }
  .kpi { background:#1a1f2e; border-radius:10px; padding:18px;
         border:1px solid #2a2f45; text-align:center; }
  .kpi .val { font-size:1.9rem; font-weight:700; color:#7eb8f7; }
  .kpi .lbl { font-size:0.75rem; color:#666; margin-top:4px;
              text-transform:uppercase; letter-spacing:0.5px; }

  /* ── FL KPI grid ── */
  .fl-grid { display:grid; grid-template-columns:repeat(4,1fr);
             gap:14px; padding:0 28px 14px; }
  .fl-kpi { background:#1a1f2e; border-radius:10px; padding:18px;
            border:1px solid #2a2f45; text-align:center; }
  .fl-kpi .val { font-size:1.9rem; font-weight:700; color:#c792ea; }
  .fl-kpi .lbl { font-size:0.75rem; color:#666; margin-top:4px;
                 text-transform:uppercase; }

  /* ── Charts row ── */
  .charts-row { display:grid; grid-template-columns:1fr 1fr;
                gap:14px; padding:0 28px 14px; }
  .chart-box { background:#1a1f2e; border-radius:10px;
               padding:16px; border:1px solid #2a2f45; }
  .chart-box h3 { font-size:0.8rem; color:#888; margin-bottom:12px;
                  text-transform:uppercase; letter-spacing:0.5px; }

  /* ── Bottom 3-col ── */
  .bottom { display:grid; grid-template-columns:1fr 1fr 1fr;
            gap:14px; padding:0 28px 28px; }

  /* ── Tables ── */
  table { width:100%; border-collapse:collapse; font-size:0.8rem; }
  th { color:#555; text-transform:uppercase; font-size:0.7rem;
       padding:7px 8px; border-bottom:1px solid #2a2f45; text-align:left; }
  td { padding:7px 8px; border-bottom:1px solid #1e2235; }
  tr:last-child td { border:none; }

  /* ── Badges ── */
  .hit   { color:#4caf50; font-weight:600; }
  .miss  { color:#f44336; font-weight:600; }
  .badge-alert { background:#ff5722; color:#fff; padding:2px 7px;
                 border-radius:10px; font-size:0.7rem; font-weight:600; }
  .badge-ok    { background:#1b5e20; color:#81c784; padding:2px 7px;
                 border-radius:10px; font-size:0.7rem; }
  .badge-fl    { background:#4a148c; color:#ce93d8; padding:2px 7px;
                 border-radius:10px; font-size:0.7rem; font-weight:600; }
  .badge-wait  { background:#1a237e; color:#90caf9; padding:2px 7px;
                 border-radius:10px; font-size:0.7rem; }

  /* ── Prob bar ── */
  .prob-bar  { height:5px; background:#2a2f45; border-radius:3px; margin-top:3px; }
  .prob-fill { height:100%; border-radius:3px;
               background:linear-gradient(90deg,#4caf50,#ff5722); }

  /* ── FL history ── */
  .fl-round-row { display:flex; justify-content:space-between;
                  align-items:center; padding:7px 0;
                  border-bottom:1px solid #1e2235; font-size:0.8rem; }
  .fl-round-row:last-child { border:none; }
</style>
</head>
<body>

<!-- Header -->
<div class="header">
  <h1>Telco-Edge CDN — ML & Federated Learning Dashboard</h1>
  <div class="header-right">
    <div class="live-badge"><div class="dot" id="agg-dot"></div>
      <span id="agg-status">Aggregator connecting...</span></div>
    <div class="live-badge"><div class="dot"></div> Live Feed</div>
  </div>
</div>

<!-- CDN Metrics -->
<div class="section-title">CDN Live Metrics</div>
<div class="kpi-grid">
  <div class="kpi"><div class="val" id="k-total">0</div>
    <div class="lbl">Total Requests</div></div>
  <div class="kpi"><div class="val" id="k-hr">0%</div>
    <div class="lbl">Cache Hit Rate</div></div>
  <div class="kpi"><div class="val" id="k-hits">0</div>
    <div class="lbl">Cache Hits</div></div>
  <div class="kpi"><div class="val" id="k-misses">0</div>
    <div class="lbl">Cache Misses</div></div>
</div>

<!-- FL Metrics -->
<div class="section-title">Federated Learning Metrics</div>
<div class="fl-grid">
  <div class="fl-kpi"><div class="val" id="fl-round">0</div>
    <div class="lbl">FL Rounds Complete</div></div>
  <div class="fl-kpi"><div class="val" id="fl-nodes">0</div>
    <div class="lbl">Nodes Participated</div></div>
  <div class="fl-kpi"><div class="val" id="fl-f1">—</div>
    <div class="lbl">Avg F1 Score</div></div>
  <div class="fl-kpi"><div class="val" id="fl-auc">—</div>
    <div class="lbl">Avg AUC-ROC</div></div>
</div>

<!-- Charts -->
<div class="charts-row">
  <div class="chart-box">
    <h3>Live Request Rate (per second)</h3>
    <canvas id="rateChart" height="110"></canvas>
  </div>
  <div class="chart-box">
    <h3>Cache Hit Rate per Edge Node</h3>
    <canvas id="nodeChart" height="110"></canvas>
  </div>
</div>

<!-- Bottom panels -->
<div class="bottom">

  <!-- Spike predictions -->
  <div class="chart-box">
    <h3>⚡ ML Spike Predictions</h3>
    <table>
      <tr><th>Video</th><th>Spike Prob</th><th>Action</th></tr>
      <tbody id="spike-table"></tbody>
    </table>
  </div>

  <!-- FL Round History -->
  <div class="chart-box">
    <h3>FL Round History</h3>
    <div id="fl-history">
      <div style="color:#555;font-size:0.8rem;padding:20px 0;text-align:center">
        Waiting for first FL round...
      </div>
    </div>
    <div style="margin-top:12px">
      <h3 style="margin-bottom:8px">Top Requested Videos</h3>
      <canvas id="videoChart" height="130"></canvas>
    </div>
  </div>

  <!-- Live feed -->
  <div class="chart-box">
    <h3>Live Request Feed</h3>
    <table>
      <tr><th>Time</th><th>Video</th><th>Node</th><th>Result</th></tr>
      <tbody id="feed-table"></tbody>
    </table>
  </div>

</div>

<script>
// ── Chart defaults ────────────────────────────────────────────────────────────
const cd = {
  plugins:{ legend:{ labels:{ color:'#aaa', font:{size:10} } } },
  scales:{
    x:{ ticks:{color:'#555'}, grid:{color:'#1e2235'} },
    y:{ ticks:{color:'#555'}, grid:{color:'#1e2235'} }
  }
}

const rateChart = new Chart(document.getElementById('rateChart'), {
  type:'line',
  data:{
    labels: Array(20).fill(''),
    datasets:[{ label:'Req/sec', data:Array(20).fill(0),
      borderColor:'#7eb8f7', backgroundColor:'rgba(126,184,247,0.1)',
      fill:true, tension:0.4, pointRadius:0 }]
  },
  options:{ ...cd, animation:false,
    scales:{ ...cd.scales, y:{...cd.scales.y, min:0} } }
})

const nodeChart = new Chart(document.getElementById('nodeChart'), {
  type:'bar',
  data:{ labels:[], datasets:[{
    label:'Hit Rate %', data:[],
    backgroundColor:'rgba(76,175,80,0.7)',
    borderColor:'#4caf50', borderWidth:1
  }]},
  options:{ ...cd, animation:{duration:300},
    scales:{ ...cd.scales, y:{...cd.scales.y, min:0, max:100} } }
})

const videoChart = new Chart(document.getElementById('videoChart'), {
  type:'bar',
  data:{ labels:[], datasets:[{
    label:'Requests', data:[],
    backgroundColor:'rgba(199,144,234,0.7)',
    borderColor:'#c792ea', borderWidth:1
  }]},
  options:{ ...cd, indexAxis:'y', animation:{duration:300},
    plugins:{ legend:{ display:false } } }
})

// ── Poll every 2 seconds ──────────────────────────────────────────────────────
async function update() {
  try {
    const d = await fetch('/api/stats').then(r => r.json())

    // CDN KPIs
    document.getElementById('k-total').textContent  = d.total_requests.toLocaleString()
    document.getElementById('k-hr').textContent     = d.cache_hit_rate + '%'
    document.getElementById('k-hits').textContent   = d.cache_hits.toLocaleString()
    document.getElementById('k-misses').textContent = d.cache_misses.toLocaleString()

    // FL KPIs
    const fl = d.fl
    document.getElementById('fl-round').textContent = fl.round
    document.getElementById('fl-nodes').textContent = fl.n_nodes || '—'
    document.getElementById('fl-f1').textContent    =
      fl.avg_f1 ? fl.avg_f1.toFixed(4) : '—'
    document.getElementById('fl-auc').textContent   =
      fl.avg_auc ? fl.avg_auc.toFixed(4) : '—'

    // Aggregator status indicator
    const dot    = document.getElementById('agg-dot')
    const aggTxt = document.getElementById('agg-status')
    if (fl.reachable) {
      dot.classList.remove('offline')
      aggTxt.textContent = `Aggregator online — Round ${fl.round}`
    } else {
      dot.classList.add('offline')
      aggTxt.textContent = 'Aggregator offline'
    }

    // Request rate chart
    rateChart.data.datasets[0].data = d.request_rate
    rateChart.update()

    // Node chart
    const nodes = Object.entries(d.node_stats)
    nodeChart.data.labels           = nodes.map(([n]) => n)
    nodeChart.data.datasets[0].data = nodes.map(([,s]) => s.hit_rate)
    nodeChart.data.datasets[0].backgroundColor = nodes.map(([,s]) =>
      s.hit_rate >= 70 ? 'rgba(76,175,80,0.7)' : 'rgba(255,87,34,0.7)')
    nodeChart.update()

    // Video chart
    videoChart.data.labels           = d.top_videos.map(([v]) => v)
    videoChart.data.datasets[0].data = d.top_videos.map(([,c]) => c)
    videoChart.update()

    // Spike table
    document.getElementById('spike-table').innerHTML =
      d.spike_alerts.map(a => `
        <tr>
          <td><b>${a.video}</b></td>
          <td>
            ${a.prob}%
            <div class="prob-bar">
              <div class="prob-fill" style="width:${Math.min(a.prob,100)}%"></div>
            </div>
          </td>
          <td>${a.alert
            ? '<span class="badge-alert">⚡ PRE-FETCH</span>'
            : '<span class="badge-ok">✓ Normal</span>'}</td>
        </tr>`).join('')

    // FL history
    if (fl.history && fl.history.length > 0) {
      document.getElementById('fl-history').innerHTML =
        [...fl.history].reverse().map(r => `
          <div class="fl-round-row">
            <span>
              <span class="badge-fl">Round ${r.round}</span>
              &nbsp; ${r.n_nodes} nodes
            </span>
            <span style="color:#888;font-size:0.75rem">
              F1 ${r.avg_f1.toFixed(3)} &nbsp; AUC ${r.avg_auc.toFixed(3)}
            </span>
          </div>`).join('')
    }

    // Live feed
    document.getElementById('feed-table').innerHTML =
      [...d.recent].reverse().map(r => `
        <tr>
          <td style="color:#555">${r.time}</td>
          <td>${r.video}</td>
          <td style="color:#666;font-size:0.75rem">${r.node}</td>
          <td class="${r.hit ? 'hit':'miss'}">${r.hit ? 'HIT':'MISS'}</td>
        </tr>`).join('')

  } catch(e) { console.error(e) }
}

update()
setInterval(update, 2000)
</script>
</body>
</html>
"""

@app.route('/')
def dashboard(): return render_template_string(HTML)

if __name__ == '__main__':
    print("Dashboard running at http://127.0.0.1:5000")
    app.run(debug=False, port=5000)