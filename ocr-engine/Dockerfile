
FROM python:3.11-slim

ENV PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PORT=8080

WORKDIR /app

# Az EasyOCR és az OpenCV futásához szükséges rendszerkönyvtárak
RUN apt-get update && apt-get install -y --no-install-recommends \
    libgl1 \
    libglib2.0-0 \
    libgomp1 \
    && rm -rf /var/lib/apt/lists/*

# Python-függőségek telepítése
COPY requirements.txt .

# CPU-only PyTorch telepítése
RUN pip install --no-cache-dir \
    torch torchvision \
    --index-url https://download.pytorch.org/whl/cpu

RUN pip install --no-cache-dir -r requirements.txt

# Alkalmazás bemásolása
COPY . .

EXPOSE 8080

CMD ["python", "main.py"]
