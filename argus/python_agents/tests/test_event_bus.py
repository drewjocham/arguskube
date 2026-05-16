from __future__ import annotations

from argus_agents.event_bus import (
    EVENT_ALERT_DETECTED,
    EVENT_ANALYSIS_COMPLETE,
    EVENT_DIAGNOSIS_COMPLETE,
    EVENT_DOCS_GENERATED,
    EventBus,
)


class TestEventBus:
    def test_on_and_emit(self) -> None:
        bus = EventBus()
        results: list[str] = []

        def handler(event: object) -> str | None:
            results.append("handled")
            return "ok"

        bus.on("test.event", handler)
        bus.emit("test.event", source="TestAgent")
        assert results == ["handled"]

    def test_no_handlers_no_error(self) -> None:
        bus = EventBus()
        event = bus.emit("nonexistent.event")
        assert event.event_type == "nonexistent.event"

    def test_multiple_handlers(self) -> None:
        bus = EventBus()
        results: list[str] = []

        def h1(event: object) -> str | None:
            results.append("h1")
            return "h1 done"

        def h2(event: object) -> str | None:
            results.append("h2")
            return "h2 done"

        bus.on("multi", h1)
        bus.on("multi", h2)
        bus.emit("multi", source="test")
        assert results == ["h1", "h2"]

    def test_registered_events(self) -> None:
        bus = EventBus()
        bus.on("a", lambda e: None)
        bus.on("b", lambda e: None)
        assert sorted(bus.registered_events()) == ["a", "b"]

    def test_clear_removes_everything(self) -> None:
        bus = EventBus()
        bus.on("x", lambda e: None)
        bus.clear()
        assert bus.registered_events() == []

    def test_results_for(self) -> None:
        bus = EventBus()
        bus.on("x", lambda e: "result1")
        bus.emit("x", source="test")
        assert bus.results_for("x") == ["result1"]
        assert bus.results_for("x") == []

    def test_on_many(self) -> None:
        bus = EventBus()
        calls: list[str] = []

        def handler(event: object) -> str | None:
            calls.append("handled")
            return None

        bus.on_many(["a", "b"], handler)
        bus.emit("a", source="test")
        bus.emit("b", source="test")
        assert len(calls) == 2

    def test_handler_exception_logged(self) -> None:
        bus = EventBus()

        def failing(event: object) -> str | None:
            raise ValueError("oops")

        bus.on("x", failing)
        event = bus.emit("x", source="test")
        assert event.event_type == "x"


class TestEventTypeConstants:
    def test_constants_are_strings(self) -> None:
        assert isinstance(EVENT_ALERT_DETECTED, str)
        assert isinstance(EVENT_ANALYSIS_COMPLETE, str)
        assert isinstance(EVENT_DIAGNOSIS_COMPLETE, str)
        assert isinstance(EVENT_DOCS_GENERATED, str)

    def test_format(self) -> None:
        assert EVENT_ALERT_DETECTED.startswith("agent.")
        assert EVENT_DIAGNOSIS_COMPLETE.startswith("agent.")
