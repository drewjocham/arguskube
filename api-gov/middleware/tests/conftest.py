from __future__ import annotations

from typing import AsyncGenerator

import httpx
import pytest
from fastapi import FastAPI


@pytest.fixture
def test_app() -> FastAPI:
    app = FastAPI()

    @app.get("/health")
    async def health():
        return {"status": "ok"}

    @app.post("/users")
    async def create_user():
        return {"id": "1", "name": "test"}

    return app
