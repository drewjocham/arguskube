from __future__ import annotations

import os

from langchain_openai import ChatOpenAI


class Settings:
    siliconflow_api_key: str = os.getenv("AGENT_SILICONFLOW_API_KEY", "")
    siliconflow_base_url: str = os.getenv("AGENT_SILICONFLOW_BASE_URL", "https://api.siliconflow.cn/v1")
    llm_model: str = os.getenv("AGENT_LLM_MODEL", "deepseek-ai/DeepSeek-V3-0324")
    embedding_model: str = os.getenv("AGENT_EMBEDDING_MODEL", "BAAI/bge-m3")
    drift_threshold: float = float(os.getenv("AGENT_DRIFT_THRESHOLD", "0.85"))
    database_url: str = os.getenv("AGENT_DATABASE_URL", "postgresql+psycopg://api-gov:api-gov@localhost:5432/api-gov")
    redis_url: str = os.getenv("AGENT_REDIS_URL", "redis://localhost:6379/0")
    server_port: int = int(os.getenv("AGENT_SERVER_PORT", "8001"))
    server_host: str = os.getenv("AGENT_SERVER_HOST", "0.0.0.0")
    log_level: str = os.getenv("AGENT_LOG_LEVEL", "info")
    otel_endpoint: str = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

    def create_llm(self, temperature: float = 0.1) -> ChatOpenAI:
        return ChatOpenAI(
            model=self.llm_model,
            api_key=self.siliconflow_api_key,
            base_url=self.siliconflow_base_url,
            temperature=temperature,
        )


config = Settings()
