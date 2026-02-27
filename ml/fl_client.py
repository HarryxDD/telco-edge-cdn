import os
import sys
import time
import pickle
import json
import requests
import threading
from pathlib import Path
from datetime import datetime

# Fix import paths
sys.path.insert(0, str(Path(__file__).parent / 'features'))
sys.path.insert(0, str(Path(__file__).parent / 'training'))

from feature_engineering import build_features, FEATURE_COLS
from train import train, predict

NODE_ID          = os.environ.get('NODE_ID',       'edge-node-01')
LOG_PATH         = os.environ.get('LOG_PATH',
    r"D:\1-Oulu-Courses\Distributed-System\Data\edge_logs.ndjson")
AGGREGATOR_URL   = os.environ.get('AGGREGATOR_URL', 'http://127.0.0.1:8092')
TRAIN_INTERVAL   = int(os.environ.get('TRAIN_INTERVAL', '120'))  
MODELS_DIR       = Path(os.environ.get('MODELS_DIR',
    r"D:\1-Oulu-Courses\Distributed-System\Data\models"))
LOCAL_MODEL_PATH = MODELS_DIR / f"local_model_{NODE_ID}.pkl"


client_state = {
    'rounds_completed' : 0,
    'last_train_time'  : None,
    'last_f1'          : None,
    'last_auc'         : None,
    'prefetch_list'    : [],
    'status'           : 'starting',
}


def send_model_to_aggregator(model, metrics: dict) -> bool:

    try:
        #Serialize model to bytes
        import io
        buf = io.BytesIO()
        pickle.dump(model, buf)
        buf.seek(0)

        response = requests.post(
            f"{AGGREGATOR_URL}/update",
            files  = {'model': ('model.pkl', buf, 'application/octet-stream')},
            data   = {
                'node_id': NODE_ID,
                'metrics': json.dumps(metrics),
            },
            timeout = 30,
        )

        if response.status_code == 200:
            data = response.json()
            print(f"Sent to aggregator - "
                  f"FL Round: {data.get('fl_round')}  "
                  f"Nodes this round: {data.get('nodes_this_round')}")
            return True
        else:
            print(f"Aggregator returned {response.status_code}: {response.text}")
            return False

    except requests.exceptions.ConnectionError:
        print(f"Cannot reach aggregator at {AGGREGATOR_URL}")
        return False
    except Exception as e:
        print(f"Error sending model: {e}")
        return False


def fetch_global_model() -> bool:

    try:
        response = requests.get(
            f"{AGGREGATOR_URL}/model",
            timeout = 30,
        )

        if response.status_code == 200:
            global_path = MODELS_DIR / "best_xgb_model.pkl"
            with open(global_path, 'wb') as f:
                f.write(response.content)
            print(f"Global model downloade {global_path}")
            return True
        else:
            print(f"Could not fetch global model: {response.status_code}")
            return False

    except requests.exceptions.ConnectionError:
        print(f"Aggregator unreachable")
        return False
    except Exception as e:
        print(f"Error fetching global model: {e}")
        return False


def check_aggregator_health() -> bool:
    try:
        r = requests.get(f"{AGGREGATOR_URL}/health", timeout=5)
        data = r.json()
        print(f"  Aggregator status : {data.get('status')}")
        print(f"  FL round          : {data.get('fl_round')}")
        print(f"  Model ready       : {data.get('model_ready')}")
        return True
    except:
        return False


def run_fl_round():

    round_num = client_state['rounds_completed'] + 1
    print(f"\n{'='*55}")
    print(f"  FL CLIENT — {NODE_ID}")
    print(f"  Round {round_num} started at {datetime.now().strftime('%H:%M:%S')}")
    print(f"{'='*55}")

    client_state['status'] = 'training'

    #Train locally
    result = train(LOG_PATH, node_id=NODE_ID)

    if result is None:
        print("Training skipped")
        client_state['status'] = 'waiting'
        return

    model   = result['model']
    metrics = result['metrics']

    client_state['last_f1']  = metrics['f1']
    client_state['last_auc'] = metrics['auc']

    print(f"\n  Sending model to aggregator at {AGGREGATOR_URL}")
    client_state['status'] = 'syncing'
    sent = send_model_to_aggregator(model, metrics)

    #Fetch global model
    if sent:
        print(f"\n  Fetching updated global model")
        fetch_global_model()

    #Generate prefetch list
    print(f"\n  Generating prefetch list")
    preds = predict(LOG_PATH, model, result['scaler'])
    prefetch = preds[preds['prefetch'] == 1]['video_id'].unique().tolist()
    client_state['prefetch_list'] = prefetch

    print(f"\n  Videos to PRE-FETCH: {prefetch[:5]}{'...' if len(prefetch)>5 else ''}")

    client_state['rounds_completed'] += 1
    client_state['last_train_time']   = datetime.now().isoformat()
    client_state['status']            = 'idle'

    print(f"\nRound {round_num} complete!")
    print(f"Next round in {TRAIN_INTERVAL}s")


def run_forever():

    print(f"\n{'='*55}")
    print(f"  FL CLIENT STARTING")
    print(f"  Node ID        : {NODE_ID}")
    print(f"  Log path       : {LOG_PATH}")
    print(f"  Aggregator     : {AGGREGATOR_URL}")
    print(f"  Train interval : {TRAIN_INTERVAL}s")
    print(f"{'='*55}")

    print(f"\n  Checking aggregator health...")
    if check_aggregator_health():
        print(f"Aggregator reachable!")
    else:
        print(f"Aggregator not reachable")

    while True:
        try:
            run_fl_round()
        except Exception as e:
            print(f"\nRound failed with error: {e}")
            client_state['status'] = 'error'

        print(f"\n  Sleeping {TRAIN_INTERVAL}s until next round")
        time.sleep(TRAIN_INTERVAL)


if __name__ == '__main__':

    mode = sys.argv[1] if len(sys.argv) > 1 else 'once'

    if mode == 'loop':
        run_forever()
    else:
        print("Running single FL round")

        if check_aggregator_health():
            print("Aggregator reachable!\n")
        else:
            print("Aggregator not reachable\n")

        run_fl_round()

        print(f"\n{'='*55}")
        print(f"  FL CLIENT SUMMARY")
        print(f"{'='*55}")
        print(f"  Node ID       : {NODE_ID}")
        print(f"  F1 Score      : {client_state['last_f1']}")
        print(f"  AUC Score     : {client_state['last_auc']}")
        print(f"  Prefetch list : {client_state['prefetch_list'][:5]}")
        print(f"  Status        : {client_state['status']}")
        print(f"{'='*55}")