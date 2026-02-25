import pandas as pd
import numpy as np

FEATURE_COLS = [
    'hour_of_day', 'day_of_week', 'is_weekend', 'is_peak_hour',
    'views_last_1h', 'views_last_3h', 'trending_score', 
    'cache_hit_rate', 'avg_response_ms', 'total_requests',
]

def load_logs(log_path):
    """Load NDJSON access logs"""
    try:
        df = pd.read_json(log_path, lines=True)
        if len(df) > 0:
            df['timestamp'] = pd.to_datetime(df['timestamp'])
        return df
    except:
        return pd.DataFrame()

def engineer_features(df):
    if len(df) < 20:
        return pd.DataFrame()
    
    # Time features
    df['hour_of_day'] = df['timestamp'].dt.hour
    df['day_of_week'] = df['timestamp'].dt.dayofweek
    df['is_weekend'] = df['day_of_week'].isin([5, 6]).astype(int)
    df['is_peak_hour'] = df['hour_of_day'].isin(range(18, 24)).astype(int)

    # Aggredate by video and hour
    df['hour_window'] = df['timestamp'].dt.floor('h')
    agg = df.groupby(['video_id', 'hour_window']).agg(
        total_requests=('video_id', 'count'),
        cache_hit_rate=('cache_hit', 'mean'),
        avg_response_ms=('response_time_ms', 'mean'),
    ).reset_index().sort_values(['video_id', 'hour_window'])

    if len(agg) < 10:
        return pd.DataFrame()

    # Rolling features
    agg['views_last_1h'] = agg.groupby('video_id')['total_requests'].shift(1).fillna(0)
    agg['views_last_3h'] = agg.groupby('video_id')['total_requests'].transform(
        lambda x: x.shift(1).rolling(window=3, min_periods=1).mean()
    ).fillna(0)
    agg['trending_score'] = (agg['views_last_1h'] / (agg['views_last_3h'] + 1e-6)).clip(0, 10)

    # Target: spike = next hour > 2x average
    video_avg = agg.groupby('video_id')['total_requests'].transform('mean')
    agg['next_hour_requests'] = agg.groupby('video_id')['total_requests'].shift(-1)
    agg['is_spike'] = (agg['next_hour_requests'] > 2 * video_avg).astype(int)

    # Add time features from hour_window
    agg['hour_of_day'] = agg['hour_window'].dt.hour
    agg['day_of_week'] = agg['hour_window'].dt.dayofweek
    agg['is_weekend'] = agg['day_of_week'].isin([5, 6]).astype(int)
    agg['is_peak_hour'] = agg['hour_of_day'].isin(range(18, 24)).astype(int)

    return agg.dropna(subset=['next_hour_requests'])