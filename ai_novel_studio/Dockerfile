FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
RUN playwright install chromium --with-deps
COPY . .
CMD ["uvicorn", "ai_novel_studio.api.main:app", "--host", "0.0.0.0", "--port", "8000"]
