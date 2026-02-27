import os
import time
import pickle
import base64

import requests
from sklearn.preprocessing import StandardScaler
from sklearn.metrics import f1_score
import xgboost as xgb
from features import load_logs, engineer_features, FEATURE_COLS

SITE_ID = os.getenv('SITE_ID', 'oulu')
ML_SERVICE_URL = os.getenv('ML_SERVICE_URL', 'http://ml-service:8090')
LOG_PATH = os.getenv('LOG_PATH', '/shared/logs/access.ndjson')
UPDATE_INTERVAL = int(os.getenv('FL_UPDATE_INTERVAL', '120'))

print(f"[FL-{SITE_ID}] Starting training client")
print(f" Log: {LOG_PATH}")
print(f" ML Service: {ML_SERVICE_URL}")
print(f" Interval: {UPDATE_INTERVAL}s")

def train_model(df):
    feat = engineer_features(df)
    if len(feat) < 10:
        return None, 0, 0

    X = feat[FEATURE_COLS].values
    y = feat['is_spike'].values

    if y.sum() == 0 or y.sum() == len(y):
        return None, 0, 0
    
    print(f"[FL-{SITE_ID}] Training on {len(feat)} samples")

    # Try to load global model for warm start (FL magic!)
    global_model_path = '/app/models/global_model.pkl'
    base_model = None
    scaler = StandardScaler()
    
    if os.path.exists(global_model_path):
        try:
            with open(global_model_path, 'rb') as f:
                global_pkg = pickle.load(f)
                base_model = global_pkg['model']
                scaler = global_pkg['scaler']
            print(f"[FL-{SITE_ID}] Loaded global model for warm start")
        except Exception as e:
            print(f"[FL-{SITE_ID}] Could not load global model: {e}")
            scaler = StandardScaler()
    else:
        print(f"[FL-{SITE_ID}] ○ No global model yet, training from scratch")

    X_scaled = scaler.fit_transform(X)

    model = xgb.XGBClassifier(
        n_estimators=100,
        max_depth=4,
        learning_rate=0.05,
        scale_pos_weight=(len(y) - y.sum()) / (y.sum() + 1),
        random_state=42,
    )

    # Warm start: continue training from global model
    if base_model is not None:
        model.fit(X_scaled, y, verbose=False, xgb_model=base_model.get_booster())
    else:
        model.fit(X_scaled, y, verbose=False)

    y_pred = model.predict(X_scaled)
    f1 = f1_score(y, y_pred, zero_division=0)

    print(f"[FL-{SITE_ID}] Training complete - F1 Score: {f1:.4f}")

    model_pkg = {
        'model': model,
        'scaler': scaler,
    }

    return model_pkg, f1, len(feat)


def send_update(model_pkg, f1, n_samples):
    """Send model to aggregator"""
    model_b64 = base64.b64encode(pickle.dumps(model_pkg)).decode()

    payload = {
        'site_id': SITE_ID,
        'model': model_b64,
        'f1': float(f1),
        'n_samples': n_samples,
    }

    try:
        resp = requests.post(f'{ML_SERVICE_URL}/update', json=payload, timeout=30)
        if resp.status_code == 200:
            print(f"[FL-{SITE_ID}] Model update sent successfully")
        else:
            print(f"[FL-{SITE_ID}] Failed to send update: {resp.status_code} {resp.text}")
    except Exception as e:
        print(f"[FL-{SITE_ID}] Error: {e}")


def download_model():
    """Download global model"""
    try:
        resp = requests.get(f'{ML_SERVICE_URL}/model', timeout=30)
        if resp.status_code == 200:
            with open('/app/models/global_model.pkl', 'wb') as f:
                f.write(resp.content)
            print(f"[FL-{SITE_ID}] Global model downloaded successfully")
            
    except:
        pass


def main():
    round_num = 0
    while True:
        round_num += 1
        print(f"\n[FL-{SITE_ID}] Round {round_num}")

        # Step 1: Download latest global model (for next round)
        download_model()
        
        # Step 2: Train on local data (warm start from global)
        df = load_logs(LOG_PATH)
        if len(df) >= 20:
            model_pkg, f1, n = train_model(df)
            if model_pkg:
                # Step 3: Send updated model to aggregator
                send_update(model_pkg, f1, n)
        else:
            print(f"[FL-{SITE_ID}] Not enough data to train (found {len(df)} samples)")

        time.sleep(UPDATE_INTERVAL)


if __name__ == '__main__':
    main()