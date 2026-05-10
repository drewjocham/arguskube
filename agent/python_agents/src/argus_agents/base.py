from __future__ import annotations

import json
import time
import uuid
from abc import ABC, abstractmethod
from datetime import datetime
from typing import Any

import httpx
from pydantic import BaseModel

from argus_agents.config import AgentConfig
from argus_agents.event_bus import EventBus, AgentEvent
from argus_agents.models import AgentResult, ChatMessage, ProactiveInsight, SessionContext


class LLMError(Exception):
    pass


class SubAgentPipeline:
    def __init__(self, parent: BaseAgent):
        self.parent = parent
        self._results: list[AgentResult] = []

    def run(self, agent_cls: type[BaseAgent], **kwargs: Any) -> AgentResult:
        agent = self.parent._sub_agent(agent_cls)
        start = time.monotonic()
        try:
            content = agent.execute(**kwargs)
            result = AgentResult(
                agent_name=agent.agent_name,
                content=content,
                duration_ms=int((time.monotonic() - start) * 1000),
            )
        except Exception as e:
            result = AgentResult(
                agent_name=agent_cls.__name__,
                success=False,
                error=str(e),
                duration_ms=int((time.monotonic() - start) * 1000),
            )
        self._results.append(result)
        return result

    def merge_results(self, separator: str = "\n\n---\n\n") -> str:
        return separator.join(r.content for r in self._results if r.success)


class BaseAgent(ABC):
    def __init__(self, config: AgentConfig | None = None):
        self.config = config or AgentConfig.from_env()
        self._client: httpx.Client | None = None
        self._sub_agents: dict[str, BaseAgent] = {}
        self.session: SessionContext | None = None
        self.bus: EventBus | None = None

    @property
    def client(self) -> httpx.Client:
        if self._client is None:
            self._client = httpx.Client(
                base_url=self.config.base_url,
                timeout=self.config.request_timeout,
            )
        return self._client

    @property
    @abstractmethod
    def system_prompt(self) -> str: ...

    @property
    def agent_name(self) -> str:
        return self.__class__.__name__

    def sub_agents(self) -> SubAgentPipeline:
        return SubAgentPipeline(self)

    def _sub_agent(self, agent_cls: type[BaseAgent]) -> BaseAgent:
        name = agent_cls.__name__
        if name not in self._sub_agents:
            agent = agent_cls(self.config)
            agent.session = self.session
            self._sub_agents[name] = agent
        return self._sub_agents[name]

    def build_messages(self, user_content: str, context: str | None = None) -> list[dict[str, str]]:
        messages: list[dict[str, str]] = []

        if self.session and self.session.conversation:
            recent = self.session.conversation[-6:]
            for msg in recent:
                if msg.role != "system":
                    messages.append({"role": msg.role, "content": msg.content})

        enriched = user_content
        if context:
            enriched = f"Context:\n{context}\n\nRequest:\n{user_content}"
        messages.append({"role": "user", "content": enriched})
        return messages

    def chat(self, messages: list[dict[str, str]]) -> str:
        payload: dict[str, Any] = {
            "model": self.config.model,
            "messages": [{"role": "system", "content": self.system_prompt}, *messages],
            "temperature": self.config.temperature,
            "max_tokens": self.config.max_tokens,
            "stream": False,
        }

        last_err: Exception | None = None
        for attempt in range(self.config.max_retries + 1):
            try:
                resp = self.client.post(
                    "/chat/completions",
                    headers={"Authorization": f"Bearer {self.config.api_key}"},
                    json=payload,
                )

                if resp.status_code == 429:
                    retry_after = int(resp.headers.get("Retry-After", 2**attempt))
                    time.sleep(retry_after)
                    continue

                resp.raise_for_status()
                data = resp.json()

                choices = data.get("choices", [])
                if not choices:
                    raise LLMError("no choices in response")

                return choices[0]["message"]["content"]

            except (httpx.HTTPError, httpx.TimeoutException) as e:
                last_err = e
                if attempt < self.config.max_retries:
                    time.sleep(2**attempt)
                    continue
                raise LLMError(f"chat failed after {self.config.max_retries} retries") from last_err

        raise LLMError("unexpected error in chat") from last_err

    def structured_chat(
        self,
        messages: list[dict[str, str]],
        response_model: type[BaseModel],
    ) -> BaseModel:
        schema = self._model_to_json_schema(response_model)
        payload = {
            "model": self.config.model,
            "messages": [{"role": "system", "content": self.system_prompt}, *messages],
            "temperature": self.config.temperature,
            "max_tokens": self.config.max_tokens,
            "stream": False,
            "response_format": {"type": "json_object", "schema": schema},
        }

        resp = self.client.post(
            "/chat/completions",
            headers={"Authorization": f"Bearer {self.config.api_key}"},
            json=payload,
        )
        resp.raise_for_status()
        data = resp.json()
        content = data["choices"][0]["message"]["content"]
        return response_model.model_validate_json(content)

    def execute(self, **kwargs: Any) -> str:
        return self.chat(self.build_messages(kwargs.get("prompt", "")))

    def track(self, message: ChatMessage) -> None:
        if self.session:
            self.session.add_message(message)

    def emit(self, event_type: str, payload: dict[str, Any] | None = None) -> AgentEvent:
        event = AgentEvent(
            event_type=event_type,
            source_agent=self.agent_name,
            payload=payload or {},
        )
        if self.bus is not None:
            self.bus.emit(event_type, source=self.agent_name, payload=payload)
        return event

    def _model_to_json_schema(self, model: type[BaseModel]) -> dict[str, Any]:
        return model.model_json_schema()

    def close(self) -> None:
        for agent in self._sub_agents.values():
            agent.close()
        if self._client is not None:
            self._client.close()
            self._client = None
