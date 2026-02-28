FROM python:3.11-slim

WORKDIR /app

RUN pip install flask xgboost scikit-learn numpy pandas imbalanced-learn prometheus-client requests

COPY app.py .
COPY models/ /app/models/

ENV MODELS_DIR=/app/models

EXPOSE 8092

CMD ["python", "app.py"]