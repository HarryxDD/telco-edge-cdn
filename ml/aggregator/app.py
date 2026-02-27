import os
import io
import time
import pickle
import threading
import json
import numpy as np
from pathlib import Path
from flask import Flask, jsonify, request, send_file
from datetime import datetime
from prometheus_client import (
    Counter, Gauge, generate_latest,
    CONTENT_TYPE_LATEST, REGISTRY
)

app = Flask(__name__)

MODELS_DIR         = Path(os.environ.get(
    'MODELS_DIR',
    r"D:\1-Oulu-Courses\Distributed-System\Data\models"
))
GLOBAL_MODEL_PATH  = MODELS_DIR / "best_xgb_model.pkl"
GLOBAL_SCALER_PATH = MODELS_DIR / "scaler.pkl"

prom_fl_rounds       = Counter('fl_rounds_total',
                                'Total FL rounds completed')
prom_fl_nodes        = Gauge('fl_participating_nodes',
                              'Number of nodes in last FL round')
prom_fl_f1           = Gauge('fl_avg_f1_score',
                              'Average F1 score of last FL round')
prom_fl_auc          = Gauge('fl_avg_auc',
                              'Average AUC score of last FL round')
prom_fl_pending      = Gauge('fl_pending_updates',
                              'Pending model updates waiting for aggregation')
prom_fl_model_ready  = Gauge('fl_model_ready',
                              '1 if global model is loaded and ready')
prom_updates_received = Counter('fl_model_updates_received_total',
                                 'Total model updates received from edge nodes',
                                 ['node_id'])

state = {
    'fl_round'           : 0,
    'participating_nodes': [],
    'node_updates'       : {},
    'global_model'       : None,
    'global_scaler'      : None,
    'round_history'      : [],
    'started_at'         : datetime.utcnow().isoformat(),
}
state_lock = threading.Lock()


def load_initial_model():
    if GLOBAL_MODEL_PATH.exists():
        with open(GLOBAL_MODEL_PATH,  'rb') as f:
            state['global_model']  = pickle.load(f)
        with open(GLOBAL_SCALER_PATH, 'rb') as f:
            state['global_scaler'] = pickle.load(f)
        prom_fl_model_ready.set(1)
        print(f"Loaded initial global model from {GLOBAL_MODEL_PATH}")
    else:
        prom_fl_model_ready.set(0)
        print("No initial model found")

load_initial_model()


def federated_average(node_updates: dict):
    if not node_updates:
        return None

    best_node    = max(node_updates.items(),
                       key=lambda x: x[1]['metrics'].get('f1', 0))
    best_model   = best_node[1]['model']
    best_node_id = best_node[0]
    best_f1      = best_node[1]['metrics'].get('f1', 0)

    avg_f1  = np.mean([v['metrics'].get('f1',  0) for v in node_updates.values()])
    avg_auc = np.mean([v['metrics'].get('auc', 0) for v in node_updates.values()])

    print(f"  FedAvg : {len(node_updates)} nodes")
    print(f"  Best   : {best_node_id} (F1={best_f1:.4f})")
    print(f"  Avg F1 : {avg_f1:.4f}  Avg AUC: {avg_auc:.4f}")

    return {
        'model'    : best_model,
        'avg_f1'   : avg_f1,
        'avg_auc'  : avg_auc,
        'best_node': best_node_id,
        'n_nodes'  : len(node_updates),
    }


def maybe_aggregate():
    with state_lock:
        updates = state['node_updates']
        if not updates:
            return

        if len(updates) >= 2:
            print(f"\n{'='*50}")
            print(f"  FL ROUND {state['fl_round'] + 1} — AGGREGATING")
            print(f"{'='*50}")

            result = federated_average(updates)

            if result:
                state['global_model'] = result['model']
                state['fl_round']    += 1

                round_summary = {
                    'round'    : state['fl_round'],
                    'timestamp': datetime.utcnow().isoformat(),
                    'n_nodes'  : result['n_nodes'],
                    'avg_f1'   : result['avg_f1'],
                    'avg_auc'  : result['avg_auc'],
                    'best_node': result['best_node'],
                    'nodes'    : list(updates.keys()),
                }
                state['round_history'].append(round_summary)

                prom_fl_rounds.inc()
                prom_fl_nodes.set(result['n_nodes'])
                prom_fl_f1.set(result['avg_f1'])
                prom_fl_auc.set(result['avg_auc'])
                prom_fl_pending.set(0)
                prom_fl_model_ready.set(1)

                MODELS_DIR.mkdir(exist_ok=True)
                with open(GLOBAL_MODEL_PATH, 'wb') as f:
                    pickle.dump(state['global_model'], f)

                state['node_updates']        = {}
                state['participating_nodes'] = []

                print(f"  Round {state['fl_round']} complete.")


# GET /health
@app.route('/health')
def health():
    return jsonify({
        'status'     : 'ok',
        'service'    : 'ml-aggregator',
        'fl_round'   : state['fl_round'],
        'uptime'     : state['started_at'],
        'model_ready': state['global_model'] is not None,
    })


# GET /model
@app.route('/model')
def get_model():
    with state_lock:
        if state['global_model'] is None:
            return jsonify({'error': 'No global model available yet'}), 404

        buf = io.BytesIO()
        pickle.dump(state['global_model'], buf)
        buf.seek(0)

        return send_file(
            buf,
            mimetype='application/octet-stream',
            as_attachment=True,
            download_name='global_model.pkl'
        )


# POST /update
@app.route('/update', methods=['POST'])
def receive_update():
    try:
        node_id     = request.form.get('node_id', 'unknown')
        metrics_str = request.form.get('metrics', '{}')
        metrics     = json.loads(metrics_str)

        model_file = request.files.get('model')
        if model_file is None:
            return jsonify({'error': 'No model file uploaded'}), 400

        model = pickle.load(model_file.stream)

        with state_lock:
            state['node_updates'][node_id] = {
                'model'    : model,
                'metrics'  : metrics,
                'timestamp': datetime.utcnow().isoformat(),
            }
            if node_id not in state['participating_nodes']:
                state['participating_nodes'].append(node_id)

            prom_fl_pending.set(len(state['node_updates']))

        prom_updates_received.labels(node_id=node_id).inc()

        print(f"\n  Received update from {node_id}")
        print(f"  F1={metrics.get('f1','N/A')}  "
              f"AUC={metrics.get('auc','N/A')}  "
              f"Samples={metrics.get('n_samples','N/A')}")

        maybe_aggregate()

        return jsonify({
            'status'          : 'received',
            'node_id'         : node_id,
            'fl_round'        : state['fl_round'],
            'nodes_this_round': len(state['node_updates']),
            'message'         : 'Aggregation triggers at 2+ nodes.',
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


# GET /metrics  -Prometheus format 
@app.route('/metrics')
def get_metrics_prometheus():
    with state_lock:
        recent = state['round_history'][-1] if state['round_history'] else {}
        prom_fl_nodes.set(recent.get('n_nodes', 0))
        prom_fl_f1.set(recent.get('avg_f1',    0))
        prom_fl_auc.set(recent.get('avg_auc',  0))
        prom_fl_pending.set(len(state['node_updates']))
        prom_fl_model_ready.set(1 if state['global_model'] else 0)

    return generate_latest(), 200, {'Content-Type': CONTENT_TYPE_LATEST}


# GET /metrics/json -JSON format 
@app.route('/metrics/json')
def get_metrics_json():
    with state_lock:
        recent_round = state['round_history'][-1] if state['round_history'] else {}
        return jsonify({
            'fl_rounds_total'    : state['fl_round'],
            'participating_nodes': state['participating_nodes'],
            'pending_updates'    : len(state['node_updates']),
            'model_ready'        : state['global_model'] is not None,
            'latest_round'       : recent_round,
            'round_history'      : state['round_history'][-5:],
        })


@app.route('/status')
def get_status():
    with state_lock:
        return jsonify({
            'fl_round'           : state['fl_round'],
            'participating_nodes': state['participating_nodes'],
            'pending_updates'    : list(state['node_updates'].keys()),
            'round_history'      : state['round_history'],
            'model_ready'        : state['global_model'] is not None,
            'started_at'         : state['started_at'],
        })


if __name__ == '__main__':
    print("=" * 50)
    print("  ML Aggregator Service")
    print("  Port      : 8092")
    print("  Endpoints :")
    print("    GET  /health")
    print("    GET  /model")
    print("    POST /update")
    print("    GET  /metrics")
    print("    GET  /metrics/json")
    print("    GET  /status")
    print("=" * 50)
    app.run(host='0.0.0.0', port=8092, debug=False)