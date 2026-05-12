"""LangChain callback for tracking LLM token usage and cost per spec/agent."""

from __future__ import annotations

import time
from typing import Any

from langchain_core.callbacks import BaseCallbackHandler
from langchain_core.outputs import LLMResult

from src.database import db


class LLMUsageCallback(BaseCallbackHandler):
    """Records every LLM call to the llm_usage table.

    Attach to ChatOpenAI:
        llm = ChatOpenAI(..., callbacks=[LLMUsageCallback(spec_id, agent)])
    """

    def __init__(self, spec_id: str, agent: str, model: str = "deepseek-ai/DeepSeek-V3-0324") -> None:
        self.spec_id = spec_id
        self.agent = agent
        self.model = model
        self._start_time: float | None = None
        self._prompt_tokens = 0
        self._completion_tokens = 0

    def on_llm_start(self, serialized: dict[str, Any], prompts: list[str], **kwargs: Any) -> None:
        self._start_time = time.time()
        # Rough estimate: 4 chars per token
        self._prompt_tokens = sum(len(p) // 4 for p in prompts)

    def on_llm_end(self, response: LLMResult, **kwargs: Any) -> None:
        if self._start_time is None:
            return
        duration_ms = int((time.time() - self._start_time) * 1000)

        # Try to get exact token counts from response
        if response.llm_output and "token_usage" in response.llm_output:
            usage = response.llm_output["token_usage"]
            prompt = usage.get("prompt_tokens", self._prompt_tokens)
            completion = usage.get("completion_tokens", 0)
        else:
            # Estimate from generated text
            prompt = self._prompt_tokens
            completion = 0
            for gen in response.generations:
                for g in gen:
                    completion += len(g.text) // 4

        import asyncio
        try:
            loop = asyncio.get_event_loop()
            if loop.is_running():
                asyncio.ensure_future(self._save(prompt, completion, duration_ms))
        except RuntimeError:
            pass

    async def _save(self, prompt_tokens: int, completion_tokens: int, duration_ms: int) -> None:
        try:
            await db.save_llm_usage(
                spec_id=self.spec_id,
                agent=self.agent,
                model=self.model,
                prompt_tokens=prompt_tokens,
                completion_tokens=completion_tokens,
                duration_ms=duration_ms,
            )
        except Exception:
            pass  # Non-critical — don't break the request
