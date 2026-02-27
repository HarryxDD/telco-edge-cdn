FROM python:3.11-slim

WORKDIR /app

RUN pip install xgboost scikit-learn numpy pandas imbalanced-learn requests

COPY features/feature_engineering.py ./feature_engineering.py
COPY training/train.py ./train.py
COPY fl_client.py .

CMD ["python", "fl_client.py", "loop"]