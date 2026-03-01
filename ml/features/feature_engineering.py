import pandas as pd
import numpy as np


#Feature columns
FEATURE_COLS = [
    'hour_of_day', 'day_of_week', 'is_weekend', 'is_peak_hour',
    'views_last_1h', 'views_last_3h', 'views_last_6h', 'trending_score',
    'cache_hit_rate', 'avg_response_ms', 'rebuffer_rate', 'avg_bitrate',
    'total_requests',
]
TARGET_COL = 'is_spike'


def load_logs(ndjson_path: str) -> pd.DataFrame:

    df = pd.read_json(ndjson_path, lines=True)
    df['timestamp'] = pd.to_datetime(df['timestamp'], utc=True)
    df = df.sort_values('timestamp').reset_index(drop=True)
    print(f"  Loaded {len(df):,} log entries from {ndjson_path}")
    return df


def aggregate_hourly(df: pd.DataFrame) -> pd.DataFrame:

    df = df.copy()
    df['hour_window'] = df['timestamp'].dt.floor('h')

    agg = (
        df
        .groupby(['video_id', 'hour_window'])
        .agg(
            total_requests  = ('video_id',               'count'),
            cache_hit_rate  = ('cache_hit',              'mean'),
            avg_response_ms = ('response_time_ms',       'mean'),
            rebuffer_rate   = ('rebuffer_event',         'mean'),
            avg_bitrate     = ('bitrate_requested_kbps', 'mean'),
        )
        .reset_index()
        .sort_values(['video_id', 'hour_window'])
        .reset_index(drop=True)
    )

    print(f"  Aggregated into {len(agg):,} hourly rows "
          f"({agg['video_id'].nunique()} videos, "
          f"{agg['hour_window'].nunique()} hours)")
    return agg


def add_time_features(df: pd.DataFrame) -> pd.DataFrame:

    df = df.copy()
    df['hour_of_day']  = df['hour_window'].dt.hour
    df['day_of_week']  = df['hour_window'].dt.dayofweek   # 0=Monday
    df['is_weekend']   = df['day_of_week'].isin([5, 6]).astype(int)
    df['is_peak_hour'] = df['hour_of_day'].isin(range(18, 24)).astype(int)
    return df


def add_rolling_features(df: pd.DataFrame) -> pd.DataFrame:

    df = df.copy().sort_values(['video_id', 'hour_window']).reset_index(drop=True)

    df['views_last_1h'] = (
        df.groupby('video_id')['total_requests']
        .shift(1)
        .fillna(0)
    )
    df['views_last_3h'] = (
        df.groupby('video_id')['total_requests']
        .transform(lambda x: x.shift(1).rolling(3, min_periods=1).mean())
        .fillna(0)
    )
    df['views_last_6h'] = (
        df.groupby('video_id')['total_requests']
        .transform(lambda x: x.shift(1).rolling(6, min_periods=1).mean())
        .fillna(0)
    )
    df['trending_score'] = (
        df['views_last_1h'] / (df['views_last_6h'] + 1e-6)
    ).round(4)

    return df


def add_spike_label(df: pd.DataFrame, multiplier: float = 2.0) -> pd.DataFrame:
    df = df.copy()
    video_avg = df.groupby('video_id')['total_requests'].transform('mean')
    df['next_hour_requests'] = df.groupby('video_id')['total_requests'].shift(-1)
    df['next_hour_requests'] = df['next_hour_requests'].fillna(df['total_requests'])
    df['is_spike'] = (df['next_hour_requests'] > multiplier * video_avg).astype(int)

    if df['is_spike'].sum() == 0:
        df.loc[df['total_requests'].nlargest(max(1, len(df)//4)).index, 'is_spike'] = 1

    return df


def build_features(ndjson_path: str,
                   include_label: bool = True) -> pd.DataFrame:

    print(f"\nBuilding features from: {ndjson_path}")

    df = load_logs(ndjson_path)
    df = aggregate_hourly(df)
    df = add_time_features(df)
    df = add_rolling_features(df)

    if include_label:
        df = add_spike_label(df)
        spike_rate = df['is_spike'].mean() * 100
        print(f"  Spike rate    : {spike_rate:.1f}%")

    print(f"  Final shape   : {df.shape}")
    print(f"  Features ready: {FEATURE_COLS}")
    return df


def get_X_y(df: pd.DataFrame):

    X = df[FEATURE_COLS].values
    y = df[TARGET_COL].values if TARGET_COL in df.columns else None
    return X, y


def get_latest_features_for_inference(ndjson_path: str,
                                       scaler) -> tuple:

    df = build_features(ndjson_path, include_label=False)
    X, _ = get_X_y(df)
    X_scaled = scaler.transform(X)
    return X_scaled, df

if __name__ == '__main__':
    import sys

    path = (sys.argv[1] if len(sys.argv) > 1
        else r"D:\1-Oulu-Courses\Distributed-System\Data\edge_logs.ndjson")

    print("=" * 55)
    print("  Feature Engineering")
    print("=" * 55)

    df = build_features(path, include_label=True)

    print(f"\nSample output (first 3 rows):")
    print(df[['video_id', 'hour_window'] + FEATURE_COLS + ['is_spike']].head(3).to_string())
