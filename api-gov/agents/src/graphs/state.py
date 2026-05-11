from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class AgentState:
    spec_id: str = ""
    spec_content: dict | None = None
    endpoints: list[dict] = field(default_factory=list)
    messages: list[dict] = field(default_factory=list)
    drift_reports: list[dict] = field(default_factory=list)
    test_cases: list[dict] = field(default_factory=list)
    errors: list[str] = field(default_factory=list)
    analysis: dict | None = None
    user_profile: dict | None = None
    spot_check_passed: bool = True
    refinement_count: int = 0
