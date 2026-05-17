from __future__ import annotations

import uuid
from datetime import datetime, timedelta
from typing import Any

from argus_agents.base import BaseAgent
from argus_agents.chroma_store import ChromaStore
from argus_agents.config import AgentConfig
from argus_agents.models import (
    ChatMessage,
    DocumentChunk,
    MemoryEntry,
    SessionContext,
    UserAction,
)


class ContextAgent(BaseAgent):
    def __init__(self, config: AgentConfig | None = None):
        super().__init__(config)
        self.store = ChromaStore(config)
        self._session: SessionContext | None = None

    @property
    def system_prompt(self) -> str:
        return "You are the Argus Context Agent — internal memory manager."

    @property
    def session(self) -> SessionContext:
        if self._session is None:
            self._session = SessionContext(
                session_id=str(uuid.uuid4()),
                user_id=self.config.user_id,
            )
        return self._session

    @session.setter
    def session(self, value: SessionContext | None) -> None:
        self._session = value

    def create_session(self, user_id: str | None = None) -> SessionContext:
        ctx = SessionContext(
            session_id=str(uuid.uuid4()),
            user_id=user_id or self.config.user_id,
        )
        self._session = ctx

        recent = self.store.get_recent_memories(limit=5)
        for mem in recent:
            ctx.metadata.setdefault("recalled_memories", []).append(mem.summary)

        return ctx

    def get_session(self) -> SessionContext:
        return self.session

    def touch_session(self) -> None:
        self.session.touch()

    def track_action(self, action: UserAction) -> None:
        self.session.add_action(action)
        self.store.store_action(action)

        if action.duration_ms and action.duration_ms > 30_000:
            self.store.store_memory(MemoryEntry(
                id=str(uuid.uuid4()),
                entry_type="fact",
                content=f"User spent {action.duration_ms/1000:.0f}s on {action.action_type.value} of {action.target}",
                summary=f"Long action: {action.action_type.value} on {action.target}",
                namespace=action.namespace,
                cluster=action.cluster,
                source_action=action.action_type,
            ))

    def track_message(self, role: str, content: str) -> None:
        msg = ChatMessage(role=role, content=content)
        self.session.add_message(msg)

        self.store.store_memory(MemoryEntry(
            id=str(uuid.uuid4()),
            entry_type="fact",
            content=f"[{role}] {content[:200]}",
            summary=f"Conversation: {role} said {content[:60]}...",
        ))

    def recall(self, query: str, n_results: int = 5) -> str:
        memories = self.store.search_memories(query, n_results=n_results)
        if not memories:
            return ""

        lines: list[str] = []
        for m in memories:
            tag = f"[{m.entry_type}]"
            ns = f" @{m.namespace}" if m.namespace else ""
            lines.append(f"- {tag}{ns} {m.summary or m.content[:120]}")

        return "Relevant context from previous sessions:\n" + "\n".join(lines)

    def store_memory(self, entry_type: str, content: str, summary: str = "", **metadata: Any) -> str:
        mem = MemoryEntry(
            id=str(uuid.uuid4()),
            entry_type=entry_type,
            content=content,
            summary=summary or content[:80],
            user_id=self.config.user_id,
            namespace=metadata.pop("namespace", None),
            cluster=metadata.pop("cluster", None),
            confidence=metadata.pop("confidence", 0.7),
            metadata=metadata,
        )
        return self.store.store_memory(mem)

    def index_document(self, document_id: str, chunks: list[dict[str, Any]]) -> int:
        doc_chunks = [
            DocumentChunk(
                chunk_id=f"{document_id}_{i}",
                document_id=document_id,
                content=c["content"],
                page_num=c.get("page_num"),
                metadata=c.get("metadata", {}),
            )
            for i, c in enumerate(chunks)
        ]
        self.store.store_document_chunks(doc_chunks)
        return len(doc_chunks)

    def query_documents(self, query: str, n_results: int = 3) -> str:
        chunks = self.store.search_documents(query, n_results=n_results)
        if not chunks:
            return ""

        lines: list[str] = []
        for c in chunks:
            lines.append(f"[{c.document_id}] {c.content[:300]}")
        return "Relevant document excerpts:\n" + "\n\n".join(lines)

    def session_summary(self) -> str:
        ctx = self.session
        lines = [
            f"Session: {ctx.session_id[:8]}",
            f"Duration: {int((datetime.now() - ctx.start_time).total_seconds() // 60)}m",
            f"Messages: {len(ctx.conversation)}",
            f"Actions: {len(ctx.recent_actions)}",
        ]
        if ctx.current_namespace:
            lines.append(f"Namespace: {ctx.current_namespace}")
        if ctx.current_cluster:
            lines.append(f"Cluster: {ctx.current_cluster}")

        if ctx.recent_actions:
            last = ctx.recent_actions[-1]
            lines.append(f"Last action: {last.action_type.value} on {last.target}")

        mem_count = self.store.count("memories")
        pat_count = self.store.count("patterns")
        lines.append(f"Stored memories: {mem_count}, patterns: {pat_count}")

        return "\n".join(lines)

    def close(self) -> None:
        self.store.close()
        super().close()
