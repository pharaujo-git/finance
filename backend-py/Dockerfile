# Build a wheel first so the runtime image carries no build tooling.
FROM python:3.13-slim AS build

WORKDIR /src
RUN pip install --no-cache-dir build==1.2.2.post1

COPY pyproject.toml README.md ./
COPY app ./app
RUN python -m build --wheel --outdir /dist


FROM python:3.13-slim

# Never write .pyc files into the layer, and keep logs unbuffered so Render's
# log tail shows startup failures immediately.
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PORT=8082

WORKDIR /app
COPY --from=build /dist/*.whl /tmp/
RUN pip install --no-cache-dir /tmp/*.whl && rm /tmp/*.whl

# An unprivileged user: nothing here needs to write to the filesystem.
RUN useradd --create-home --uid 10001 api
USER api

EXPOSE 8082

# $PORT is set by the platform; the shell form is what expands it.
CMD ["sh", "-c", "exec uvicorn app.main:build --factory --host 0.0.0.0 --port ${PORT}"]
