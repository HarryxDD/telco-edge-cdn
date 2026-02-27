FROM python:3.11-slim

WORKDIR /app

RUN pip install flask xgboost scikit-learn numpy pandas imbalanced-learn prometheus-client requests

COPY app.py .
COPY init_models.sh .
COPY models/ /app/models/

RUN chmod +x init_models.sh

ENV MODELS_DIR=/app/models

EXPOSE 8092

CMD ["./init_models.sh"]