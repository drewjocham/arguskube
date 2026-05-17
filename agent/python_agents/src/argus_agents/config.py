from pathlib import Path

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class AgentConfig(BaseSettings):
    api_key: str = Field(default="", alias="ARGUS_API_KEY")
    base_url: str = Field(default="https://api.deepseek.com/v1", alias="ARGUS_API_BASE_URL")
    model: str = Field(default="deepseek-chat", alias="ARGUS_MODEL")
    temperature: float = Field(default=0.3, le=2.0, ge=0.0)
    max_tokens: int = Field(default=4096, ge=1)
    request_timeout: int = Field(default=120, ge=1)
    max_retries: int = Field(default=3, ge=0, le=10)

    chroma_path: str = Field(
        default=str(Path.home() / ".argus" / "chroma"),
        alias="ARGUS_CHROMA_PATH",
    )
    session_ttl_minutes: int = Field(default=60, alias="ARGUS_SESSION_TTL")
    proactive_interval_seconds: int = Field(default=30, alias="ARGUS_PROACTIVE_INTERVAL")
    user_id: str = Field(default="default", alias="ARGUS_USER_ID")

    model_config = SettingsConfigDict(env_file=".env", extra="ignore", populate_by_name=True)

    @classmethod
    def from_env(cls) -> "AgentConfig":
        return cls()
