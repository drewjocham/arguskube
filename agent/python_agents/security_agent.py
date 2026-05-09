from __future__ import annotations

from pydantic import BaseModel, Field

from .base import BaseAgent
from .event_bus import EVENT_SECURITY_SCAN_COMPLETE


class SecurityFinding(BaseModel):
    resource: str
    issue: str
    severity: str = "medium"
    cve: str | None = None
    fix_command: str = ""
    namespace: str | None = None


class SecurityOutput(BaseModel):
    risk_score: int = Field(ge=0, le=100)
    critical_findings: list[SecurityFinding]
    patch_priority: list[str]
    compliance_gaps: list[str]
    summary: str = ""


class CveLookupAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return "You are a CVE lookup sub-agent. Prioritise CVEs by CVSS score, exploitability, and whether a patch exists."

    def analyse(self, report: str) -> str:
        return self.chat([{"role": "user", "content": f"Prioritise these CVEs:\n{report}"}])


class RbacAuditAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are an RBAC audit sub-agent. Check for over-permissive roles,
wildcard verbs, cluster-admin bindings to service accounts, and privilege escalation paths."""

    def audit(self, rbac: str) -> str:
        return self.chat([{"role": "user", "content": f"Audit this RBAC config:\n{rbac}"}])


class SecurityAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are the Argus Security Agent — a Kubernetes security specialist.

Your sole responsibility is **identifying and triaging security issues** in the cluster.

Rules:
1. Analyse vulnerability scan results (CVEs) — patch Critical and High first.
2. Check for misconfigurations: privileged containers, hostNetwork, hostPID.
3. Review RBAC for over-permissive roles and wildcard verbs.
4. Flag outdated k8s versions.
5. Check network policies.
6. Prioritise: actively exploited > internet-facing > critical CVE > high CVE > misconfig.
"""

    def assess(self, vuln_report: str, rbac_audit: str | None = None, namespace: str | None = None) -> SecurityOutput:
        pipeline = self.sub_agents()

        pipeline.run(CveLookupAgent, prompt=f"Analyse these CVEs:\n{vuln_report}")
        if rbac_audit:
            pipeline.run(RbacAuditAgent, prompt=f"Audit this RBAC:\n{rbac_audit}")

        enriched = vuln_report
        sub_results = pipeline.merge_results()
        if sub_results:
            enriched = f"Vulnerability Report:\n{vuln_report}\n\nSub-agent analysis:\n{sub_results}"
        if rbac_audit:
            enriched += f"\n\nRBAC Audit:\n{rbac_audit}"

        ns_context = f" in namespace '{namespace}'" if namespace else ""
        result = self.structured_chat(
            [{"role": "user", "content": f"Assess the security posture{ns_context}:\n{enriched}"}],
            SecurityOutput,
        )
        self.emit(EVENT_SECURITY_SCAN_COMPLETE, {
            "risk_score": result.risk_score,
            "critical_count": len([f for f in result.critical_findings if f.severity in ("critical", "high")]),
            "namespace": namespace,
        })
        return result

    def assess_freeform(self, vuln_report: str, rbac_audit: str | None = None, namespace: str | None = None) -> str:
        messages = []
        ns_context = f" in namespace '{namespace}'" if namespace else ""
        messages.append({
            "role": "user",
            "content": f"Assess the security posture of the cluster{ns_context}:\n\nVulnerability Report:\n{vuln_report}"
        })
        if rbac_audit:
            messages.append({"role": "user", "content": f"RBAC Audit:\n{rbac_audit}"})
        return self.chat(messages)
