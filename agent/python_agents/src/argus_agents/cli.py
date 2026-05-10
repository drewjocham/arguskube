from __future__ import annotations

import argparse
import logging
import sys
from typing import NoReturn

from argus_agents._version import __version__

logging.basicConfig(
    level=logging.WARNING,
    format="%(levelname)s [%(name)s] %(message)s",
)


def _setup_logging(verbose: bool) -> None:
    level = logging.DEBUG if verbose else logging.WARNING
    logging.getLogger("argus_agents").setLevel(level)


def cmd_version(args: argparse.Namespace) -> None:
    print(f"argus-agents {__version__}")
    sys.exit(0)


def cmd_interactive(args: argparse.Namespace) -> None:
    _setup_logging(args.verbose)
    from argus_agents.orchestrator import OrchestratorAgent

    agent = OrchestratorAgent()
    agent.start_session()

    print("Argus Agents interactive session. Type 'exit' or Ctrl-D to quit.")
    while True:
        try:
            user_input = input("> ").strip()
        except (EOFError, KeyboardInterrupt):
            print()
            break
        if not user_input:
            continue
        if user_input.lower() in ("exit", "quit"):
            break
        result = agent.route(user_input)
        print(result)

    agent.close()


def cmd_diagnose(args: argparse.Namespace) -> None:
    _setup_logging(args.verbose)
    from argus_agents.diagnosis_agent import DiagnosisAgent

    agent = DiagnosisAgent()
    result = agent.diagnose(args.alert, args.context)
    print(result.model_dump_json(indent=2))


def cmd_analyze(args: argparse.Namespace) -> None:
    _setup_logging(args.verbose)
    from argus_agents.analysis_agent import AnalysisAgent

    agent = AnalysisAgent()
    result = agent.analyze(args.metrics, args.window)
    print(result.model_dump_json(indent=2))


def cmd_remediate(args: argparse.Namespace) -> None:
    _setup_logging(args.verbose)
    from argus_agents.remediation_agent import RemediationAgent

    agent = RemediationAgent()
    result = agent.remediate(args.diagnosis, args.context)
    print(result.model_dump_json(indent=2))


def cmd_secure(args: argparse.Namespace) -> None:
    _setup_logging(args.verbose)
    from argus_agents.security_agent import SecurityAgent

    agent = SecurityAgent()
    result = agent.assess(args.vuln_report, args.rbac, args.namespace)
    print(result.model_dump_json(indent=2))


def cmd_cost(args: argparse.Namespace) -> None:
    _setup_logging(args.verbose)
    from argus_agents.cost_agent import CostAgent

    agent = CostAgent()
    result = agent.optimize(args.usage, args.pricing)
    print(result.model_dump_json(indent=2))


def cmd_docs(args: argparse.Namespace) -> None:
    _setup_logging(args.verbose)
    from argus_agents.docs_agent import DocsAgent

    agent = DocsAgent()
    if args.type == "runbook":
        result = agent.generate_runbook(args.topic, args.context)
    else:
        result = agent.generate_postmortem(args.summary, args.timeline, args.impact)
    print(result.model_dump_json(indent=2))


def cmd_serve(args: argparse.Namespace) -> None:
    _setup_logging(args.verbose)
    from argus_agents.orchestrator import OrchestratorAgent

    agent = OrchestratorAgent()
    agent.start_session()

    from http.server import HTTPServer, BaseHTTPRequestHandler
    import json

    class AgentHandler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:
            length = int(self.headers.get("Content-Length", 0))
            body = self.rfile.read(length).decode()
            payload = json.loads(body) if body else {}
            user_input = payload.get("input", "")
            result = agent.route(user_input)
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"result": result}).encode())

        def log_message(self, fmt: str, *args: tuple) -> None:
            pass

    server = HTTPServer(("0.0.0.0", args.port), AgentHandler)
    print(f"Serving on http://0.0.0.0:{args.port}")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print()
    finally:
        agent.close()
        server.server_close()


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(
        prog="argus-agent",
        description="Argus multi-agent AI system for Kubernetes operations",
    )
    parser.add_argument("--verbose", "-v", action="store_true", help="Enable debug logging")
    parser.add_argument("--version", action="store_true", help="Show version and exit")
    sub = parser.add_subparsers(dest="command")

    sub.add_parser("interactive", help="Start an interactive session")

    diag = sub.add_parser("diagnose", help="Run root cause analysis")
    diag.add_argument("alert", help="Alert description or incident summary")
    diag.add_argument("--context", help="Optional cluster context")

    ana = sub.add_parser("analyze", help="Analyze cluster metrics")
    ana.add_argument("metrics", help="Metrics snapshot")
    ana.add_argument("--window", default="1h", help="Time window for analysis")

    rem = sub.add_parser("remediate", help="Generate remediation steps")
    rem.add_argument("diagnosis", help="Diagnosis to remediate")
    rem.add_argument("--context", help="Optional issue context")

    sec = sub.add_parser("secure", help="Assess security posture")
    sec.add_argument("vuln_report", help="Vulnerability scan report")
    sec.add_argument("--rbac", help="Optional RBAC audit data")
    sec.add_argument("--namespace", help="Optional namespace scope")

    cost = sub.add_parser("cost", help="Analyze infrastructure costs")
    cost.add_argument("usage", help="Usage report")
    cost.add_argument("--pricing", help="Optional node pricing data")

    doc = sub.add_parser("docs", help="Generate documentation")
    doc.add_argument("type", choices=["runbook", "postmortem"], help="Document type")
    doc.add_argument("topic", nargs="?", help="Alert type or document topic")
    doc.add_argument("--context", help="Optional context")
    doc.add_argument("--summary", help="Incident summary (for postmortem)")
    doc.add_argument("--timeline", help="Incident timeline (for postmortem)")
    doc.add_argument("--impact", help="Incident impact (for postmortem)")

    serve = sub.add_parser("serve", help="Start HTTP server")
    serve.add_argument("--port", type=int, default=8080, help="Server port")

    args = parser.parse_args(argv)

    if args.version:
        return cmd_version(args)

    if not args.command:
        parser.print_help()
        sys.exit(1)

    command_map = {
        "interactive": cmd_interactive,
        "diagnose": cmd_diagnose,
        "analyze": cmd_analyze,
        "remediate": cmd_remediate,
        "secure": cmd_secure,
        "cost": cmd_cost,
        "docs": cmd_docs,
        "serve": cmd_serve,
    }
    command_map[args.command](args)


if __name__ == "__main__":
    main()
