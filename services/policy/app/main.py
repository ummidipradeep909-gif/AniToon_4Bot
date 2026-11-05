from contextlib import asynccontextmanager
import asyncpg
from fastapi import FastAPI, Header, HTTPException
from pydantic import BaseModel, Field
from redis.asyncio import Redis
from .ai import analyze_message
from .config import settings
from .security import require_service_token

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

app = FastAPI(title="AniToon_4Bot Policy Service", version="0.4.0", lifespan=lifespan)

class MessageSignal(BaseModel):
    chat_id: int
    user_id: int
    message_id: int
    text: str = Field(default="", max_length=4096)

class AIAnalysisRequest(BaseModel):
    chat_id: int
    user_id: int
    message_id: int
    feature: str = Field(default="anti_spam", min_length=1, max_length=40)
    text: str = Field(min_length=1, max_length=4000)

@app.get("/health")
async def health():
    try:
        async with app.state.pg.acquire() as conn:
            await conn.fetchval("SELECT 1")
        await app.state.redis.ping()
    except Exception as exc:
        raise HTTPException(status_code=503, detail="dependency unavailable") from exc
    return {"status": "ok", "service": "policy", "database": "ok", "redis": "ok",
            "ai_enabled": bool(settings.ai_enabled and settings.openai_api_key and settings.ai_service_token)}

@app.post("/v1/policy/evaluate")
async def evaluate(signal: MessageSignal, x_ai_service_token: str = Header(default="")):
    require_service_token(x_ai_service_token, settings.ai_service_token)
    key = f"rate:{signal.chat_id}:{signal.user_id}"
    count = await app.state.redis.incr(key)
    if count == 1:
        await app.state.redis.expire(key, 10)
    return {"decision": "allow", "signals": {"messages_in_10s": count},
            "enforcement_enabled": False}

@app.post("/v1/ai/analyze")
async def ai_analyze(request: AIAnalysisRequest, x_ai_service_token: str = Header(default="")):
    # Keep provider spend private to trusted bot services.
    require_service_token(x_ai_service_token, settings.ai_service_token)
    if not settings.ai_enabled or not settings.openai_api_key:
        raise HTTPException(status_code=503, detail="AI assistance is not configured")
    try:
        result = await analyze_message(request.text, api_key=settings.openai_api_key,
                                       model=settings.openai_model, feature=request.feature,
                                       timeout_seconds=settings.ai_timeout_seconds)
    except RuntimeError as exc:
        raise HTTPException(status_code=502, detail="AI analysis unavailable") from exc
    return {"chat_id": request.chat_id, "user_id": request.user_id,
            "message_id": request.message_id, "feature": request.feature, "mode": "assist",
            "analysis": result.model_dump(), "enforcement_enabled": False}
