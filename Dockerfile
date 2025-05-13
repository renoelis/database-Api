FROM python:3.11-slim

WORKDIR /app

# 安装编译工具和依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    g++ \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/*

# 复制依赖文件并安装依赖
COPY requirements.txt .

# 先安装一些可能需要编译的关键包
RUN pip install --no-cache-dir wheel setuptools
RUN pip install --no-cache-dir chardet

# 替换cchardet为chardet（在requirements.txt中注释掉cchardet）
RUN grep -v "cchardet" requirements.txt > requirements_filtered.txt && \
    pip install --no-cache-dir -r requirements_filtered.txt

# 复制应用代码
COPY . .

# 创建配置目录
RUN mkdir -p /app/config

# 设置环境变量
ENV PYTHONPATH=/app
ENV PORT=3010

# 暴露端口
EXPOSE 3010

# 使用Gunicorn运行应用
CMD ["gunicorn", "app.main:app", "--workers", "4", "--worker-class", "uvicorn.workers.UvicornWorker", "--bind", "0.0.0.0:3010"] 