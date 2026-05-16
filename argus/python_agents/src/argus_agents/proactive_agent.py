from __future__ import annotations

import threading
import time
import uuid
from collections import Counter, defaultdict
from datetime import datetime, timedelta
from typing import Any

from argus_agents.base import BaseAgent
from argus_agents.chroma_store import ChromaStore
from argus_agents.config import AgentConfig
from argus_agents.models import (
    HabitPattern,
    ProactiveInsight,
    UserAction,
)


class ProactiveAgent(BaseAgent):
    def __init__(self, config: AgentConfig | None = None):
        super().__init__(config)
        self.store = ChromaStore(config)
        self._insights: list[ProactiveInsight] = []
        self._running = False
        self._thread: threading.Thread | None = None
        self._action_buffer: list[UserAction] = []
        self._on_insight_callback: Any = None

    @property
    def system_prompt(self) -> str:
        return "You are the Argus Proactive Agent — internal pattern analysis."

    def start(self, callback: Any = None) -> None:
        self._running = True
        self._on_insight_callback = callback
        self._thread = threading.Thread(target=self._loop, daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._running = False
        if self._thread:
            self._thread.join(timeout=5)

    def is_running(self) -> bool:
        return self._running

    def observe(self, action: UserAction) -> None:
        self._action_buffer.append(action)
        self.store.store_action(action)
        if len(self._action_buffer) > 200:
            self._action_buffer = self._action_buffer[-200:]

    def _loop(self) -> None:
        while self._running:
            try:
                self._process_cycle()
            except Exception:
                pass
            time.sleep(self.config.proactive_interval_seconds)

    def _process_cycle(self) -> None:
        if not self._action_buffer:
            return

        window = self._action_buffer[-50:] if len(self._action_buffer) > 50 else self._action_buffer

        patterns = self._detect_temporal_patterns(window)
        patterns += self._detect_sequential_patterns(window)
        patterns += self._detect_frequency_patterns(window)

        for pattern in patterns:
            self.store.store_pattern(pattern)

        insights = self._generate_insights(window, patterns)
        for ins in insights:
            self._insights.append(ins)
            if self._on_insight_callback:
                self._on_insight_callback(ins)

    def _detect_temporal_patterns(self, actions: list[UserAction]) -> list[HabitPattern]:
        by_hour: dict[int, list[UserAction]] = defaultdict(list)
        for a in actions:
            by_hour[a.timestamp.hour].append(a)

        patterns: list[HabitPattern] = []
        for hour, acts in by_hour.items():
            if len(acts) < 3:
                continue
            targets = [a.target for a in acts]
            common = Counter(targets).most_common(1)
            if common:
                target, count = common[0]
                pct = count / len(acts)
                if pct > 0.4:
                    patterns.append(HabitPattern(
                        id=str(uuid.uuid4()),
                        pattern_type="temporal",
                        description=f"User frequently checks {target} around {hour}:00",
                        predicted_next_action=f"User likely to check {target}",
                        confidence=round(pct, 2),
                        observation_count=len(acts),
                        last_observed=datetime.now(),
                        trigger_conditions={"hour": hour, "target": target},
                    ))
        return patterns

    def _detect_sequential_patterns(self, actions: list[UserAction]) -> list[HabitPattern]:
        if len(actions) < 4:
            return []

        patterns: list[HabitPattern] = []
        pairs: Counter = Counter()
        for i in range(len(actions) - 1):
            key = (actions[i].target, actions[i + 1].target)
            pairs[key] += 1

        for (first, second), count in pairs.most_common(5):
            if count >= 3:
                total = sum(1 for a in actions if a.target == first)
                confidence = round(count / max(total, 1), 2)
                if confidence > 0.3:
                    patterns.append(HabitPattern(
                        id=str(uuid.uuid4()),
                        pattern_type="sequential",
                        description=f"After checking {first}, user usually checks {second}",
                        predicted_next_action=f"Pre-fetch {second} status",
                        confidence=confidence,
                        observation_count=count,
                        last_observed=datetime.now(),
                        trigger_conditions={"after_target": first, "predict_target": second},
                    ))
        return patterns

    def _detect_frequency_patterns(self, actions: list[UserAction]) -> list[HabitPattern]:
        by_target: dict[str, list[UserAction]] = defaultdict(list)
        for a in actions:
            by_target[a.target].append(a)

        patterns: list[HabitPattern] = []
        for target, acts in by_target.items():
            if len(acts) >= 5:
                types = [a.action_type.value for a in acts]
                common_type = Counter(types).most_common(1)[0][0]
                patterns.append(HabitPattern(
                    id=str(uuid.uuid4()),
                    pattern_type="frequency",
                    description=f"User frequently performs {common_type} on {target} ({len(acts)} times)",
                    predicted_next_action=f"User may {common_type} {target} again",
                    confidence=round(min(len(acts) / 20, 0.95), 2),
                    observation_count=len(acts),
                    last_observed=datetime.now(),
                    trigger_conditions={"target": target, "action_type": common_type},
                ))
        return patterns

    def _generate_insights(self, actions: list[UserAction], patterns: list[HabitPattern]) -> list[ProactiveInsight]:
        insights: list[ProactiveInsight] = []

        for p in patterns:
            if p.confidence < 0.4:
                continue

            if p.pattern_type == "temporal":
                insights.append(ProactiveInsight(
                    id=str(uuid.uuid4()),
                    trigger_pattern=p.description,
                    pattern_id=p.id,
                    confidence=p.confidence,
                    suggestion=f"I notice you often check {p.trigger_conditions.get('target', 'resources')} around this time. I've pre-fetched the latest state for you.",
                    rationale=f"Observed pattern: {p.description} over {p.observation_count} occurrences",
                    action_type="prefetch",
                    auto_executed=True,
                    requires_approval=False,
                ))

            elif p.pattern_type == "sequential":
                predict = p.trigger_conditions.get("predict_target", "")
                if predict:
                    insights.append(ProactiveInsight(
                        id=str(uuid.uuid4()),
                        trigger_pattern=p.description,
                        pattern_id=p.id,
                        confidence=p.confidence,
                        suggestion=f"I see you just checked {p.trigger_conditions.get('after_target', '')}. Would you like me to pull up {predict} as well?",
                        rationale=f"Sequential pattern: {p.description} (seen {p.observation_count}x)",
                        action_type="suggest",
                        auto_executed=False,
                        requires_approval=True,
                    ))

        recent = actions[-10:] if len(actions) >= 10 else actions
        targets_seen = set(a.target for a in recent)
        for target in targets_seen:
            same = [a for a in recent if a.target == target]
            if len(same) >= 3 and any(a.action_type.value == "view" for a in same):
                ns = same[-1].namespace
                if ns:
                    insights.append(ProactiveInsight(
                        id=str(uuid.uuid4()),
                        trigger_pattern=f"Repeated views of {target} in {ns}",
                        confidence=0.5,
                        suggestion=f"Scanning namespace '{ns}' for changes on {target}...",
                        rationale=f"User viewed {target} {len(same)}x recently. Pre-checking for status changes.",
                        action_type="prefetch",
                        auto_executed=True,
                        requires_approval=False,
                    ))

        for a in actions[-5:]:
            if a.action_type.value == "diagnose":
                insights.append(ProactiveInsight(
                    id=str(uuid.uuid4()),
                    trigger_pattern="Recent diagnosis activity",
                    confidence=0.4,
                    suggestion=f"I noticed you've been diagnosing issues. I've indexed the last 100 log entries for faster retrieval.",
                    rationale="Diagnosis activity detected — preparing log context",
                    action_type="precompute",
                    auto_executed=True,
                    requires_approval=False,
                ))
                break

        if len(actions) >= 2:
            last = actions[-1]
            prev = actions[-2]
            if last.namespace and prev.namespace and last.namespace == prev.namespace:
                insights.append(ProactiveInsight(
                    id=str(uuid.uuid4()),
                    trigger_pattern=f"Focused work in namespace {last.namespace}",
                    confidence=0.6,
                    suggestion=f"You're working in '{last.namespace}'. I'll keep an eye on resource changes there.",
                    rationale=f"Multiple recent actions in namespace {last.namespace}",
                    action_type="notify",
                    auto_executed=True,
                    requires_approval=False,
                ))

        insights.sort(key=lambda x: x.confidence, reverse=True)
        return insights[:5]

    def pending_insights(self, clear: bool = True) -> list[ProactiveInsight]:
        results = list(self._insights)
        if clear:
            self._insights.clear()
        return results

    def latest_insight(self) -> ProactiveInsight | None:
        if self._insights:
            return self._insights[-1]
        return None

    def report_activity(self) -> str:
        if not self._action_buffer:
            return ""

        recent = self._action_buffer[-5:]
        lines: list[str] = []
        for a in recent:
            ns = f" in {a.namespace}" if a.namespace else ""
            lines.append(f"  \u00b7 {a.action_type.value} on {a.target}{ns}")

        patterns = self.store.get_patterns(min_confidence=0.3)
        if patterns:
            lines.append("")
            lines.append("  Patterns detected:")
            for p in patterns[:3]:
                lines.append(f"    \u00b7 {p.description} (confidence: {p.confidence:.0%})")

        insights = self._insights[-3:]
        if insights:
            lines.append("")
            lines.append("  Recent insights:")
            for ins in insights:
                lines.append(f"    \u00b7 {ins.suggestion}")

        return "\n".join(lines)

    def close(self) -> None:
        self.stop()
        self.store.close()
        super().close()
