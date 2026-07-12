FROM python:3.13-slim

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

# Sets the working directory
WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy project's all files to the working directory
COPY . .

# Expose the port for the application
EXPOSE 8000

# Allows access to the container from outside by binding to 0.0.0.0:8000
CMD ["gunicorn", "api_server.wsgi:application", "--bind", "0.0.0.0:8000"]
