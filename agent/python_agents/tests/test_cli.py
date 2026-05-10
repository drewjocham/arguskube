from __future__ import annotations

from typing import Any
from unittest.mock import patch

import pytest

from argus_agents.cli import main


class TestCLI:
    def test_version(self, capsys: Any) -> None:
        with pytest.raises(SystemExit) as exc:
            main(["--version"])
        assert exc.value.code == 0
        captured = capsys.readouterr()
        assert "0.1.0" in captured.out

    def test_help(self, capsys: Any) -> None:
        with pytest.raises(SystemExit) as exc:
            main(["--help"])
        assert exc.value.code == 0

    def test_interactive_stops_on_exit(self, capsys: Any) -> None:
        with patch("builtins.input", side_effect=["exit"]):
            main(["--verbose", "interactive"])

    def test_diagnose_subcommand(self, capsys: Any) -> None:
        main(["--verbose", "diagnose", "pod crash"])
        captured = capsys.readouterr()
        assert captured.out.strip()

    def test_analyze_subcommand(self, capsys: Any) -> None:
        main(["--verbose", "analyze", "cpu high"])
        captured = capsys.readouterr()
        assert captured.out.strip()

    def test_remediate_subcommand(self, capsys: Any) -> None:
        main(["--verbose", "remediate", "oomkilled"])
        captured = capsys.readouterr()
        assert captured.out.strip()

    def test_secure_subcommand(self, capsys: Any) -> None:
        main(["--verbose", "secure", "cve report"])
        captured = capsys.readouterr()
        assert captured.out.strip()

    def test_cost_subcommand(self, capsys: Any) -> None:
        main(["--verbose", "cost", "usage data"])
        captured = capsys.readouterr()
        assert captured.out.strip()

    def test_docs_runbook_subcommand(self, capsys: Any) -> None:
        main(["--verbose", "docs", "runbook", "Pod CrashLoopBackOff"])
        captured = capsys.readouterr()
        assert captured.out.strip()

    def test_docs_postmortem_subcommand(self, capsys: Any) -> None:
        main(["--verbose", "docs", "postmortem", "--summary", "outage", "--timeline", "10:00", "--impact", "high"])
        captured = capsys.readouterr()
        assert captured.out.strip()
