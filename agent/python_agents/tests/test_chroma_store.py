from __future__ import annotations

from typing import Any

import pytest

from argus_agents.chroma_store import ChromaStore
from argus_agents.models import (
    DocumentChunk,
    HabitPattern,
    MemoryEntry,
    UserAction,
    ActionType,
)


class TestChromaStore:
    def test_initialise(self, config: Any) -> None:
        store = ChromaStore(config)
        assert store is not None
        assert store.count("memories") == 0
        store.close()

    def test_store_and_search_memory(self, config: Any) -> None:
        store = ChromaStore(config)
        entry = MemoryEntry(
            id="m1",
            entry_type="fact",
            content="pod nginx restarted 3 times today",
            summary="nginx restarts",
            namespace="production",
        )
        store.store_memory(entry)
        assert store.count("memories") == 1

        results = store.search_memories("nginx", n_results=10)
        assert any("nginx" in r.content or "nginx" in r.summary for r in results)

        store.delete_collection("memories")
        store.close()

    def test_get_recent_memories(self, config: Any) -> None:
        store = ChromaStore(config)
        for i in range(5):
            store.store_memory(MemoryEntry(
                id=f"m{i}", entry_type="fact", content=f"memory {i}"
            ))
        recent = store.get_recent_memories(limit=10)
        assert len(recent) == 5
        store.delete_collection("memories")
        store.close()

    def test_store_and_search_actions(self, config: Any) -> None:
        store = ChromaStore(config)
        action = UserAction(
            action_type=ActionType.view,
            target="deployment/nginx",
            namespace="production",
        )
        store.store_action(action)

        results = store.search_actions("nginx", n_results=10)
        assert any(a.target == "deployment/nginx" for a in results)

        store.delete_collection("actions")
        store.close()

    def test_store_and_get_patterns(self, config: Any) -> None:
        store = ChromaStore(config)
        pattern = HabitPattern(
            id="p1",
            pattern_type="temporal",
            description="user checks nginx at 9am",
            confidence=0.85,
            observation_count=10,
        )
        store.store_pattern(pattern)
        patterns = store.get_patterns(min_confidence=0.3)
        assert any(p.id == "p1" for p in patterns)

        store.delete_collection("patterns")
        store.close()

    def test_store_document_chunks(self, config: Any) -> None:
        store = ChromaStore(config)
        chunks = [
            DocumentChunk(chunk_id="doc1_0", document_id="doc1", content="kubernetes pod lifecycle"),
            DocumentChunk(chunk_id="doc1_1", document_id="doc1", content="pod scheduling and eviction"),
        ]
        ids = store.store_document_chunks(chunks)
        assert len(ids) == 2

        results = store.search_documents("pod lifecycle", n_results=10)
        assert len(results) > 0

        store.delete_collection("documents")
        store.close()

    def test_search_with_no_results(self, config: Any) -> None:
        store = ChromaStore(config)
        results = store.search_memories("nonexistent", n_results=5)
        assert results == []
        store.close()

    def test_count_zero_for_empty_collection(self, config: Any) -> None:
        store = ChromaStore(config)
        store.delete_collection("memories")
        assert store.count("memories") == 0
        store.close()

    def test_delete_nonexistent_collection(self, config: Any) -> None:
        store = ChromaStore(config)
        store.delete_collection("nonexistent")
        store.close()
