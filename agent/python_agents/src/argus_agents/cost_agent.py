from __future__ import annotations

from pydantic import BaseModel, Field

from argus_agents.base import BaseAgent
from argus_agents.event_bus import EVENT_COST_ANALYSIS_COMPLETE


class SavingsOpportunity(BaseModel):
    resource: str
    waste_type: str
    current_cost_monthly: float = 0.0
    estimated_savings_monthly: float = 0.0
    recommendation: str = ""


class RightSizeRecommendation(BaseModel):
    resource: str
    current_requests: str = ""
    suggested_requests: str = ""
    rationale: str = ""


class CostOutput(BaseModel):
    monthly_burn_rate: float = 0.0
    top_savings: list[SavingsOpportunity]
    rightsizing: list[RightSizeRecommendation]
    spot_eligible_workloads: list[str]
    summary: str = ""


class ResourceUsageAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are a resource usage analysis sub-agent. Identify idle resources,
zero-CPU pods, nodes under 30% utilisation, and over-provisioned requests."""

    def analyse(self, usage: str) -> str:
        return self.chat([{"role": "user", "content": f"Analyse this usage report for waste:\n{usage}"}])


class PricingAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are a cloud pricing sub-agent. Given usage patterns and pricing data,
estimate costs and identify savings from spot instances, reserved instances,
and right-sizing."""

    def estimate(self, usage: str, pricing: str) -> str:
        return self.chat([{"role": "user", "content": f"Usage:\n{usage}\n\nPricing:\n{pricing}"}])


class CostAgent(BaseAgent):
    @property
    def system_prompt(self) -> str:
        return """You are the Argus Cost Agent — a FinOps specialist for Kubernetes.

Your sole responsibility is **analysing and optimising cloud infrastructure costs**.

Rules:
1. Identify idle or underutilised resources: zero-CPU pods, nodes <30% utilised.
2. Flag over-provisioned requests.
3. Recommend right-sizing based on actual usage.
4. Detect orphaned resources: unattached volumes, unused LBs, old snapshots.
5. Suggest spot instance candidates.
6. Estimate monthly savings for each recommendation.
"""

    def optimize(self, usage_report: str, node_pricing: str | None = None) -> CostOutput:
        pipeline = self.sub_agents()

        pipeline.run(ResourceUsageAgent, prompt=f"Analyse this usage:\n{usage_report}")
        if node_pricing:
            pipeline.run(PricingAgent, prompt=f"Usage:\n{usage_report}\n\nPricing:\n{node_pricing}")

        enriched = usage_report
        sub_results = pipeline.merge_results()
        if sub_results:
            enriched = f"{usage_report}\n\nSub-agent analysis:\n{sub_results}"

        result = self.structured_chat(
            [{"role": "user", "content": f"Analyse this cluster's cost efficiency and suggest optimizations:\n{enriched}"}],
            CostOutput,
        )
        self.emit(EVENT_COST_ANALYSIS_COMPLETE, {
            "burn_rate": result.monthly_burn_rate,
            "savings_count": len(result.top_savings),
            "top_saving": result.top_savings[0].recommendation if result.top_savings else "",
        })
        return result

    def optimize_freeform(self, usage_report: str, node_pricing: str | None = None) -> str:
        messages = [
            {"role": "user", "content": f"Analyse this cluster's cost efficiency and suggest optimizations:\n{usage_report}"}
        ]
        if node_pricing:
            messages.append({"role": "user", "content": f"Node pricing data:\n{node_pricing}"})
        return self.chat(messages)
