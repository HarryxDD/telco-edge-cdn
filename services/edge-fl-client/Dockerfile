FROM python:3.11-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY features.py train.py ./

RUN mkdir -p /app/models /shared/logs

CMD ["python", "-u", "train.py"]