#!/bin/bash
if [ ! -f /app/models/best_xgb_model.pkl ]; then
    echo "No model found"
fi
python app.py