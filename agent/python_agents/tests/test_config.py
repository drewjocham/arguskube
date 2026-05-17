from __future__ import annotations

from typing import Any

from argus_agents.config import AgentConfig


class TestAgentConfig:
    def test_from_env_returns_instance(self) -> None:
        cfg = AgentConfig.from_env()
        assert isinstance(cfg, AgentConfig)

    def test_default_values(self) -> None:
        cfg = AgentConfig()
        assert cfg.api_key == ""
        assert cfg.base_url == "https://api.deepseek.com/v1"
        assert cfg.model == "deepseek-chat"
        assert cfg.temperature == 0.3
        assert cfg.max_tokens == 4096
        assert cfg.request_timeout == 120
        assert cfg.max_retries == 3

    def test_custom_values(self) -> None:
        cfg = AgentConfig(
            api_key="sk-test",
            base_url="http://localhost:8000/v1",
            model="gpt-4",
            temperature=0.7,
            max_tokens=2048,
            request_timeout=30,
            max_retries=5,
            chroma_path="/custom/path",
        )
        assert cfg.api_key == "sk-test"
        assert cfg.base_url == "http://localhost:8000/v1"
        assert cfg.temperature == 0.7
        assert cfg.chroma_path == "/custom/path"

    def test_temperature_bounds(self) -> None:
        cfg = AgentConfig(temperature=0.0)
        assert cfg.temperature == 0.0
        cfg = AgentConfig(temperature=2.0)
        assert cfg.temperature == 2.0

    def test_max_retries_bounds(self) -> None:
        cfg = AgentConfig(max_retries=0)
        assert cfg.max_retries == 0
        cfg = AgentConfig(max_retries=10)
        assert cfg.max_retries == 10
