import pickle
import time
import sys
import os
import numpy as np
import pandas as pd
from pathlib import Path
from sklearn.preprocessing   import StandardScaler
from sklearn.metrics         import f1_score, roc_auc_score, accuracy_score
from sklearn.model_selection import train_test_split
from imblearn.over_sampling  import SMOTE
import xgboost as xgb

sys.path.insert(0, str(Path(__file__).parent.parent / 'features'))
from feature_engineering import (
    build_features, get_X_y, FEATURE_COLS, TARGET_COL
)

DATA_PATH         = os.environ.get('LOG_PATH',
    r"D:\1-Oulu-Courses\Distributed-System\Data\edge_logs.ndjson")
MODELS_DIR        = Path(os.environ.get('MODELS_DIR',
    r"D:\1-Oulu-Courses\Distributed-System\Data\models"))
GLOBAL_MODEL_PATH  = MODELS_DIR / "best_xgb_model.pkl"
GLOBAL_SCALER_PATH = MODELS_DIR / "scaler.pkl"
LOCAL_MODEL_PATH   = MODELS_DIR / "local_model.pkl"
LOCAL_SCALER_PATH  = MODELS_DIR / "local_scaler.pkl"

XGB_PARAMS = {
    'n_estimators'     : 300,
    'max_depth'        : 4,
    'learning_rate'    : 0.05,
    'use_label_encoder': False,
    'eval_metric'      : 'logloss',
    'random_state'     : 42,
}


def load_global_model():

    if GLOBAL_MODEL_PATH.exists():
        with open(GLOBAL_MODEL_PATH,  'rb') as f: model  = pickle.load(f)
        with open(GLOBAL_SCALER_PATH, 'rb') as f: scaler = pickle.load(f)
        print(f"  Loaded global model from {GLOBAL_MODEL_PATH}")
        return model, scaler
    else:
        print("  No global model found")
        return None, None


def train(log_path: str,
          node_id: str = "edge-node-local",
          min_samples: int = 50) -> dict:
    
    print(f"\n{'='*55}")
    print(f"  LOCAL TRAINING — {node_id}")
    print(f"{'='*55}")
    start_time = time.time()

    # Feature Engineering
    print("\n1. Engineering features")
    df = build_features(log_path, include_label=True)

    if len(df) < min_samples:
        print(f"Only {len(df)} samples, need at least {min_samples} to train.")
        print(f"Skipping training for this round.")
        return None

    X, y = get_X_y(df)
    print(f"  X shape: {X.shape}  |  Spikes: {y.sum()} ({y.mean()*100:.1f}%)")

    #Scale features
    print("\n2. Scaling features")
    _, global_scaler = load_global_model()

    if global_scaler is not None:
        scaler = global_scaler
        X_scaled = scaler.transform(X)
        print("  Using global scaler")
    else:
        scaler = StandardScaler()
        X_scaled = scaler.fit_transform(X)
        print("  Fitted new local scaler")

    #Train/test split
    print("\n3. Splitting")
    split_idx  = int(len(X_scaled) * 0.8)
    X_train, X_test = X_scaled[:split_idx], X_scaled[split_idx:]
    y_train, y_test = y[:split_idx],        y[split_idx:]
    print(f"  Train: {X_train.shape}  |  Test: {X_test.shape}")

    # SMOTE for class imbalance 
    print("\n4. SMOTE")
    pos = y_train.sum()
    neg = (y_train == 0).sum()

    if pos >= 2:
        k = min(3, int(pos) - 1)
        smote = SMOTE(random_state=42, k_neighbors=k)
        X_train_res, y_train_res = smote.fit_resample(X_train, y_train)
        print(f"Before SMOTE - 0: {neg}  1: {pos}")
        print(f"After  SMOTE - 0: {(y_train_res==0).sum()}"
              f"1: {(y_train_res==1).sum()}")
    else:
        print(f"  Too few spike samples ({pos}) for SMOTE")
        X_train_res, y_train_res = X_train, y_train

    #XGBoost
    print("\n5. Training XGBoost")
    model = xgb.XGBClassifier(**XGB_PARAMS)
    model.fit(X_train_res, y_train_res)

    #Evaluation
    y_prob = model.predict_proba(X_test)[:, 1]
    y_pred = (y_prob >= 0.3).astype(int)

    metrics = {
        'node_id'  : node_id,
        'accuracy' : round(accuracy_score(y_test, y_pred), 4),
        'f1'       : round(f1_score(y_test, y_pred, zero_division=0), 4),
        'auc'      : round(roc_auc_score(y_test, y_prob), 4),
        'n_samples': len(df),
        'n_spikes' : int(y.sum()),
        'train_time': round(time.time() - start_time, 2),
    }

    print(f"\n  Results:")
    print(f"    Accuracy  : {metrics['accuracy']*100:.1f}%")
    print(f"    F1 Score  : {metrics['f1']:.4f}")
    print(f"    AUC-ROC   : {metrics['auc']:.4f}")
    print(f"    Train time: {metrics['train_time']}s")

    MODELS_DIR.mkdir(exist_ok=True)
    with open(LOCAL_MODEL_PATH,  'wb') as f: pickle.dump(model,  f)
    with open(LOCAL_SCALER_PATH, 'wb') as f: pickle.dump(scaler, f)

    print(f"\n Local model saved to: {LOCAL_MODEL_PATH}")

    return {
        'model'  : model,
        'scaler' : scaler,
        'metrics': metrics,
    }


def predict(log_path: str,
            model=None,
            scaler=None) -> pd.DataFrame:
    
    from feature_engineering import build_features, FEATURE_COLS

    if model is None or scaler is None:
        with open(LOCAL_MODEL_PATH,  'rb') as f: model  = pickle.load(f)
        with open(LOCAL_SCALER_PATH, 'rb') as f: scaler = pickle.load(f)

    df       = build_features(log_path, include_label=False)
    X        = df[FEATURE_COLS].values
    X_scaled = scaler.transform(X)
    probs    = model.predict_proba(X_scaled)[:, 1]
    preds    = (probs >= 0.3).astype(int)

    results = df[['video_id', 'hour_window']].copy()
    results['spike_prob']  = probs.round(4)
    results['prefetch']    = preds
    results = results.sort_values('spike_prob', ascending=False)

    return results

if __name__ == '__main__':
    import sys

    log_path = (sys.argv[1] if len(sys.argv) > 1 else DATA_PATH)
    node_id  = (sys.argv[2] if len(sys.argv) > 2 else "edge-node-01")

    #Train
    result = train(log_path, node_id=node_id)

    if result:
        print(f"\n{'='*55}")
        print(f"  SPIKE PREDICTIONS - Top 10")
        print(f"{'='*55}")
        preds = predict(log_path, result['model'], result['scaler'])
        print(preds.head(10).to_string(index=False))

        prefetch_list = preds[preds['prefetch'] == 1]['video_id'].tolist()
        print(f"\n  Videos to PRE-FETCH now : {prefetch_list}")