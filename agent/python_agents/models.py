from __future__ import annotations

from datetime import datetime
from enum import Enum
from typing import Any, Literal

from pydantic import BaseModel, Field


class Severity(str, Enum):
    critical = "critical"
    high = "high"
    medium = "medium"
    low = "low"
    info = "info"


class ActionType(str, Enum):
    query = "query"
    command = "command"
    view = "view"
    edit = "edit"
    diagnose = "diagnose"
    remediate = "remediate"
    analyze = "analyze"
    security_scan = "security_scan"
    cost_check = "cost_check"
    docs = "docs"


class UserAction(BaseModel):
    action_type: ActionType
    target: str
    namespace: str | None = None
    cluster: str | None = None
    timestamp: datetime = Field(default_factory=datetime.now)
    duration_ms: int | None = None
    tokens_used: int | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


class ChatMessage(BaseModel):
    role: Literal["system", "user", "assistant", "tool"]
    content: str
    timestamp: datetime = Field(default_factory=datetime.now)
    tool_calls: list[dict[str, Any]] | None = None


class SessionContext(BaseModel):
    session_id: str
    user_id: str
    start_time: datetime = Field(default_factory=datetime.now)
    last_active: datetime = Field(default_factory=datetime.now)
    current_namespace: str | None = None
    current_cluster: str | None = None
    conversation: list[ChatMessage] = Field(default_factory=list)
    recent_actions: list[UserAction] = Field(default_factory=list)
    active_document_ids: list[str] = Field(default_factory=list)
    metadata: dict[str, Any] = Field(default_factory=dict)

    def touch(self) -> None:
        self.last_active = datetime.now()

    def add_action(self, action: UserAction) -> None:
        self.recent_actions.append(action)
        if len(self.recent_actions) > 100:
            self.recent_actions = self.recent_actions[-100:]
        self.touch()

    def add_message(self, message: ChatMessage) -> None:
        self.conversation.append(message)
        if len(self.conversation) > 50:
            self.conversation = self.conversation[-50:]
        self.touch()


class MemoryEntry(BaseModel):
    id: str
    entry_type: Literal["pattern", "fact", "preference", "incident", "workflow", "habit"]
    content: str
    summary: str = ""
    user_id: str = "default"
    namespace: str | None = None
    cluster: str | None = None
    source_action: ActionType | None = None
    confidence: float = Field(default=0.5, ge=0.0, le=1.0)
    timestamp: datetime = Field(default_factory=datetime.now)
    expiry: datetime | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


class DocumentChunk(BaseModel):
    chunk_id: str
    document_id: str
    content: str
    page_num: int | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


class HabitPattern(BaseModel):
    id: str
    pattern_type: Literal["temporal", "sequential", "contextual", "frequency"]
    description: str
    trigger_conditions: dict[str, Any] = Field(default_factory=dict)
    predicted_next_action: str = ""
    confidence: float = Field(default=0.0, ge=0.0, le=1.0)
    observation_count: int = 1
    first_observed: datetime = Field(default_factory=datetime.now)
    last_observed: datetime = Field(default_factory=datetime.now)
    metadata: dict[str, Any] = Field(default_factory=dict)


class ProactiveInsight(BaseModel):
    id: str
    trigger_pattern: str
    pattern_id: str | None = None
    confidence: float
    suggestion: str
    rationale: str
    action_type: Literal["notify", "prefetch", "precompute", "suggest"]
    auto_executed: bool = False
    requires_approval: bool = True
    result_summary: str | None = None
    timestamp: datetime = Field(default_factory=datetime.now)
    acknowledged: bool = False


class AgentResult(BaseModel):
    agent_name: str
    success: bool = True
    content: str = ""
    structured: dict[str, Any] | None = None
    sub_agent_results: list[AgentResult] = Field(default_factory=list)
    events_emitted: list[str] = Field(default_factory=list)
    auto_spawned: list[str] = Field(default_factory=list)
    confidence: float = Field(default=0.5, ge=0.0, le=1.0)
    duration_ms: int = 0
    error: str | None = None
    insights: list[ProactiveInsight] = Field(default_factory=list)


class TaskClassification(BaseModel):
    task_type: Literal["diagnose", "remediate", "analyze", "secure", "cost", "docs", "chat", "unknown"]
    reasoning: str = ""
    confidence: float = Field(default=0.0, ge=0.0, le=1.0)
    requires_sub_agents: list[str] = Field(default_factory=list)
    urgency: Literal["low", "medium", "high", "critical"] = "medium"
