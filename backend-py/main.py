"""Vercel entry point.

Vercel's Python runtime serves an ASGI application exported as `app`, so the
same FastAPI instance `uvicorn app.main:build` runs is handed over directly and
nothing here starts a server -- Vercel owns the socket.

The pool needs no special sizing: `create_pool` opens `min_size=1` eagerly and
only grows under concurrency, and a serverless instance serves one request at a
time, so it settles at a single connection. Neon's pooler does the fanning out.
"""

from app.main import create_app

app = create_app()
