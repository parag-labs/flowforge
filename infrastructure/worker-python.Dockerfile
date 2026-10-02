FROM python:3.12-slim
WORKDIR /app
COPY services/worker-python/worker.py ./
ENTRYPOINT ["python", "worker.py"]
