from __future__ import annotations

import json
import uuid
from datetime import datetime
from pathlib import Path
from typing import Any, Literal

import chromadb
from chromadb.config import Settings

from .config import AgentConfig
from .models import DocumentChunk, HabitPattern, MemoryEntry, UserAction


class ChromaStore:
    def __init__(self, config: AgentConfig | None = None):
        self.config = config or AgentConfig.from_env()
        db_path = Path(self.config.chroma_path)
        db_path.mkdir(parents=True, exist_ok=True)

        self.client = chromadb.PersistentClient(
            path=str(db_path),
            settings=Settings(anonymized_telemetry=False),
        )

        self._collections: dict[str, chromadb.Collection] = {}

    def _collection(self, name: str) -> chromadb.Collection:
        if name not in self._collections:
            self._collections[name] = self.client.get_or_create_collection(
                name=f"argus_{name}",
                metadata={"hnsw:space": "cosine"},
            )
        return self._collections[name]

    # ── Memory ────────────────────────────────────────────────────

    def store_memory(self, entry: MemoryEntry, embedding: list[float] | None = None) -> str:
        col = self._collection("memories")
        doc_id = entry.id or str(uuid.uuid4())
        col.upsert(
            ids=[doc_id],
            documents=[entry.content],
            embeddings=[embedding] if embedding else None,
            metadatas=[{
                "entry_type": entry.entry_type,
                "user_id": entry.user_id,
                "namespace": entry.namespace or "",
                "cluster": entry.cluster or "",
                "summary": entry.summary,
                "confidence": str(entry.confidence),
                "timestamp": entry.timestamp.isoformat(),
                "expiry": entry.expiry.isoformat() if entry.expiry else "",
            }],
        )
        return doc_id

    def search_memories(
        self,
        query: str,
        n_results: int = 5,
        entry_type: str | None = None,
        namespace: str | None = None,
    ) -> list[MemoryEntry]:
        col = self._collection("memories")
        where: dict[str, Any] = {}
        if entry_type:
            where["entry_type"] = entry_type
        if namespace:
            where["namespace"] = namespace

        results = col.query(
            query_texts=[query],
            n_results=n_results,
            where=where or None,
        )
        return self._results_to_memories(results)

    def get_recent_memories(self, limit: int = 20) -> list[MemoryEntry]:
        col = self._collection("memories")
        results = col.get(limit=limit)
        return self._results_to_memories(results)

    # ── Actions ───────────────────────────────────────────────────

    def store_action(self, action: UserAction) -> str:
        col = self._collection("actions")
        doc_id = str(uuid.uuid4())
        content = f"{action.action_type.value} on {action.target}"
        col.upsert(
            ids=[doc_id],
            documents=[content],
            metadatas=[{
                "action_type": action.action_type.value,
                "target": action.target,
                "namespace": action.namespace or "",
                "cluster": action.cluster or "",
                "timestamp": action.timestamp.isoformat(),
                "duration_ms": str(action.duration_ms or 0),
                "metadata": json.dumps(action.metadata),
            }],
        )
        return doc_id

    def search_actions(
        self,
        query: str,
        n_results: int = 10,
        action_type: str | None = None,
    ) -> list[UserAction]:
        col = self._collection("actions")
        where = {"action_type": action_type} if action_type else None
        results = col.query(query_texts=[query], n_results=n_results, where=where)
        actions: list[UserAction] = []
        for i, _ in enumerate(results["ids"][0]):
            meta = results["metadatas"][0][i]
            actions.append(UserAction(
                action_type=meta.get("action_type", "query"),
                target=meta.get("target", ""),
                namespace=meta.get("namespace") or None,
                cluster=meta.get("cluster") or None,
                timestamp=datetime.fromisoformat(meta.get("timestamp", datetime.now().isoformat())),
                duration_ms=int(meta.get("duration_ms", 0)) or None,
                metadata=json.loads(meta.get("metadata", "{}")),
            ))
        return actions

    # ── Habits / Patterns ─────────────────────────────────────────

    def store_pattern(self, pattern: HabitPattern) -> str:
        col = self._collection("patterns")
        col.upsert(
            ids=[pattern.id],
            documents=[pattern.description],
            metadatas=[{
                "pattern_type": pattern.pattern_type,
                "confidence": str(pattern.confidence),
                "observation_count": str(pattern.observation_count),
                "first_observed": pattern.first_observed.isoformat(),
                "last_observed": pattern.last_observed.isoformat(),
                "predicted_next": pattern.predicted_next_action,
                "trigger_conditions": json.dumps(pattern.trigger_conditions),
            }],
        )
        return pattern.id

    def get_patterns(self, min_confidence: float = 0.3) -> list[HabitPattern]:
        col = self._collection("patterns")
        results = col.get()
        patterns: list[HabitPattern] = []
        for i, doc_id in enumerate(results["ids"]):
            meta = results["metadatas"][i]
            if float(meta.get("confidence", "0")) < min_confidence:
                continue
            patterns.append(HabitPattern(
                id=doc_id,
                pattern_type=meta.get("pattern_type", "temporal"),
                description=results["documents"][i] or "",
                predicted_next_action=meta.get("predicted_next", ""),
                confidence=float(meta.get("confidence", "0")),
                observation_count=int(meta.get("observation_count", "1")),
                first_observed=datetime.fromisoformat(meta.get("first_observed", datetime.now().isoformat())),
                last_observed=datetime.fromisoformat(meta.get("last_observed", datetime.now().isoformat())),
                trigger_conditions=json.loads(meta.get("trigger_conditions", "{}")),
            ))
        return patterns

    # ── Documents ─────────────────────────────────────────────────

    def store_document_chunks(self, chunks: list[DocumentChunk]) -> list[str]:
        col = self._collection("documents")
        ids = [c.chunk_id for c in chunks]
        col.upsert(
            ids=ids,
            documents=[c.content for c in chunks],
            metadatas=[{
                "document_id": c.document_id,
                "page_num": str(c.page_num or 0),
                "metadata": json.dumps(c.metadata),
            } for c in chunks],
        )
        return ids

    def search_documents(self, query: str, n_results: int = 5) -> list[DocumentChunk]:
        col = self._collection("documents")
        results = col.query(query_texts=[query], n_results=n_results)
        chunks: list[DocumentChunk] = []
        for i, _ in enumerate(results["ids"][0]):
            meta = results["metadatas"][0][i]
            chunks.append(DocumentChunk(
                chunk_id=results["ids"][0][i],
                document_id=meta.get("document_id", ""),
                content=results["documents"][0][i],
                page_num=int(meta.get("page_num", 0)) or None,
                metadata=json.loads(meta.get("metadata", "{}")),
            ))
        return chunks

    # ── Helpers ───────────────────────────────────────────────────

    def _results_to_memories(self, results: Any) -> list[MemoryEntry]:
        entries: list[MemoryEntry] = []
        for i, doc_id in enumerate(results["ids"][0] if results.get("ids") and results["ids"][0] else []):
            meta = results["metadatas"][0][i]
            entries.append(MemoryEntry(
                id=doc_id,
                entry_type=meta.get("entry_type", "fact"),
                content=results["documents"][0][i],
                summary=meta.get("summary", ""),
                user_id=meta.get("user_id", "default"),
                namespace=meta.get("namespace") or None,
                cluster=meta.get("cluster") or None,
                confidence=float(meta.get("confidence", "0.5")),
                timestamp=datetime.fromisoformat(meta.get("timestamp", datetime.now().isoformat())),
                expiry=datetime.fromisoformat(meta["expiry"]) if meta.get("expiry") else None,
            ))
        return entries

    def count(self, collection: str) -> int:
        col = self._collection(collection)
        return col.count()

    def delete_collection(self, collection: str) -> None:
        try:
            self.client.delete_collection(f"argus_{collection}")
            self._collections.pop(collection, None)
        except ValueError:
            pass

    def close(self) -> None:
        self.client.clear_system_cache()
