import os
from pathlib import Path

from pydantic import Field
from pydantic_settings import BaseSettings


class AgentConfig(BaseSettings):
    api_key: str = Field(default="", validation_alias="ARGUS_API_KEY")
    base_url: str = Field(default="https://api.deepseek.com/v1", validation_alias="ARGUS_API_BASE_URL")
    model: str = Field(default="deepseek-chat", validation_alias="ARGUS_MODEL")
    temperature: float = Field(default=0.3, le=2.0, ge=0.0)
    max_tokens: int = Field(default=4096, ge=1)
    request_timeout: int = Field(default=120, ge=1)
    max_retries: int = Field(default=3, ge=0, le=10)

    chroma_path: str = Field(
        default=str(Path.home() / ".argus" / "chroma"),
        validation_alias="ARGUS_CHROMA_PATH",
    )
    session_ttl_minutes: int = Field(default=60, validation_alias="ARGUS_SESSION_TTL")
    proactive_interval_seconds: int = Field(default=30, validation_alias="ARGUS_PROACTIVE_INTERVAL")
    user_id: str = Field(default="default", validation_alias="ARGUS_USER_ID")

    model_config = {"env_file": ".env", "extra": "ignore"}

    @classmethod
    def from_env(cls) -> "AgentConfig":
        return cls()
