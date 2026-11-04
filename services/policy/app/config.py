from pydantic_settings import BaseSettings, SettingsConfigDict

class Settings(BaseSettings):
    database_url: str
    redis_url: str
    policy_service_host: str = "0.0.0.0"
    policy_service_port: int = 8000
    ai_enabled: bool = False
    ai_provider: str = "openai"
    openai_api_key: str = ""
    openai_model: str = "gpt-4o-mini"
    ai_timeout_seconds: float = 12.0
    model_config = SettingsConfigDict(env_file=".env", extra="ignore")

settings = Settings()
