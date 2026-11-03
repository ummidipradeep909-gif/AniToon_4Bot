from contextlib import asynccontextmanager

import asyncpg
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from redis.asyncio import Redis

from .config import settings


@asynccontextmanager
async def lifespan(app: FastAPI):
    app.state.pg = await asyncpg.create_pool(settings.database_url, min_size=1, max_size=5)
    app.state.redis = Redis.from_url(settings.redis_url, decode_responses=True)
    try:
        await app.state.redis.ping()
        yield
    finally:
        await app.state.redis.aclose()
        await app.state.pg.close()


app = FastAPI(title="AniToon_4Bot Policy Service", version="0.1.0", lifespan=lifespan)


class MessageSignal(BaseModel):
    chat_id: int
    user_id: int
    message_id: int
    text: str = Field(default="", max_length=4096)


@app.get("/health")
async def health():
    try:
        async with app.state.pg.acquire() as conn:
            await conn.fetchval("SELECT 1")
        await app.state.redis.ping()
    except Exception as exc:
        raise HTTPException(status_code=503, detail="dependency unavailable") from exc
    return {"status": "ok", "service": "policy", "database": "ok", "redis": "ok"}


@app.post("/v1/policy/evaluate")
async def evaluate(signal: MessageSignal):
    # Telemetry only. No user is punished until moderation thresholds,
    # admin overrides, and audit persistence are implemented and tested.
    key = f"rate:{signal.chat_id}:{signal.user_id}"
    count = await app.state.redis.incr(key)
    if count == 1:
        await app.state.redis.expire(key, 10)
    return {
        "decision": "allow",
        "signals": {"messages_in_10s": count},
        "enforcement_enabled": False,
    }
