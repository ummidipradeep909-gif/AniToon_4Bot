import os
from pathlib import Path
from dotenv import dotenv_values
from pydantic_settings import BaseSettings, SettingsConfigDict

# Preserve the existing local .env workflow before resolving NAME_FILE overrides.
for _key, _value in dotenv_values(".env").items():
    if _value is not None:
        os.environ.setdefault(_key, _value)


def _load_secret_file(name: str, *, required: bool = False) -> None:
    direct = os.environ.get(name, "").strip()
    secret_path = os.environ.get(f"{name}_FILE", "").strip()
    if direct and secret_path:
        raise RuntimeError(f"configure either {name} or {name}_FILE, not both")
    if secret_path:
        try:
            value = Path(secret_path).read_text(encoding="utf-8").strip()
        except OSError:
            raise RuntimeError(f"{name} could not be loaded from its configured secret file") from None
        if required and not value:
            raise RuntimeError(f"{name} is required and its secret file is empty")
        os.environ[name] = value
    elif required and not direct:
        raise RuntimeError(f"{name} is required (set {name} or {name}_FILE)")


_load_secret_file("DATABASE_URL", required=True)
_load_secret_file("REDIS_URL", required=True)
_load_secret_file("AI_SERVICE_TOKEN", required=os.getenv("AI_ENABLED", "false").lower() == "true")
_load_secret_file("OPENAI_API_KEY", required=False)


class Settings(BaseSettings):
    database_url: str
    redis_url: str
    policy_service_host: str = "0.0.0.0"
    policy_service_port: int = 8000
    ai_enabled: bool = False
    ai_provider: str = "openai"
    ai_service_token: str = ""
    openai_api_key: str = ""
    openai_model: str = "gpt-4o-mini"
    ai_timeout_seconds: float = 12.0
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")


settings = Settings()
