"""Thin wrapper around the vast.ai REST API.

We avoid the official `vastai` Python SDK because (a) it shells out to its own
CLI internally and (b) its surface changes between releases. The HTTP API is
stable and documented at https://vast.ai/docs/api.

Authentication: pass an API key from https://cloud.vast.ai/account/ via the
VAST_API_KEY env var.
"""

from __future__ import annotations

import json
import os
import re
import string
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import requests
import yaml

API_BASE = "https://console.vast.ai/api/v0"
DEFAULT_TIMEOUT = 30


class VastError(RuntimeError):
    """Raised when the vast.ai API returns an error response."""


@dataclass
class Offer:
    id: int
    gpu_name: str
    num_gpus: int
    dph_total: float       # dollars per hour, all-in
    reliability: float
    disk_gb: float
    inet_down_mbps: float
    machine_id: int

    @classmethod
    def from_api(cls, raw: dict[str, Any]) -> "Offer":
        return cls(
            id=raw["id"],
            gpu_name=raw.get("gpu_name", "?"),
            num_gpus=raw.get("num_gpus", 1),
            dph_total=raw.get("dph_total", 0.0),
            reliability=raw.get("reliability2", 0.0),
            disk_gb=raw.get("disk_space", 0.0),
            inet_down_mbps=raw.get("inet_down", 0.0),
            machine_id=raw.get("machine_id", 0),
        )


@dataclass
class Instance:
    id: int
    status: str
    public_ip: str
    ssh_port: int | None
    exposed_ports: dict[str, int]   # internal port -> public port
    label: str
    gpu_name: str
    dph_total: float

    @classmethod
    def from_api(cls, raw: dict[str, Any]) -> "Instance":
        # vast.ai surfaces ports under different keys depending on lifecycle stage.
        ports = raw.get("ports") or {}
        exposed = {}
        # ports look like {"8000/tcp": [{"HostIp": "0.0.0.0", "HostPort": "33012"}]}
        for k, v in ports.items():
            if not v:
                continue
            internal = k.split("/")[0]
            exposed[internal] = int(v[0]["HostPort"])
        return cls(
            id=raw["id"],
            status=raw.get("actual_status", raw.get("intended_status", "?")),
            public_ip=raw.get("public_ipaddr", ""),
            ssh_port=raw.get("ssh_port"),
            exposed_ports=exposed,
            label=raw.get("label", "") or "",
            gpu_name=raw.get("gpu_name", "?"),
            dph_total=raw.get("dph_total", 0.0),
        )


class VastClient:
    def __init__(self, api_key: str | None = None):
        self.api_key = api_key or os.environ.get("VAST_API_KEY")
        if not self.api_key:
            raise VastError(
                "VAST_API_KEY is not set. Get one at "
                "https://cloud.vast.ai/account/ and `export VAST_API_KEY=…`."
            )

    def _headers(self) -> dict[str, str]:
        return {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }

    def _request(self, method: str, path: str, **kwargs) -> Any:
        url = f"{API_BASE}{path}"
        kwargs.setdefault("timeout", DEFAULT_TIMEOUT)
        kwargs.setdefault("headers", {}).update(self._headers())
        resp = requests.request(method, url, **kwargs)
        if not resp.ok:
            raise VastError(f"{method} {path} -> {resp.status_code}: {resp.text[:400]}")
        if resp.text.strip():
            return resp.json()
        return None

    # ----- Offers --------------------------------------------------------------

    def search_offers(self, query: dict[str, Any]) -> list[Offer]:
        """Search the marketplace. `query` follows vast.ai's filter syntax."""
        body = {"q": query}
        raw = self._request("PUT", "/bundles/", data=json.dumps(body))
        return [Offer.from_api(o) for o in (raw.get("offers") or [])]

    # ----- Instances ----------------------------------------------------------

    def create_instance(
        self,
        offer_id: int,
        image: str,
        onstart_cmd: str,
        env: dict[str, str] | None = None,
        label: str = "",
        disk_gb: int = 60,
    ) -> int:
        """Rent the offer and provision an instance. Returns the instance id."""
        body: dict[str, Any] = {
            "client_id": "me",
            "image": image,
            "disk": disk_gb,
            "label": label,
            "onstart": onstart_cmd,
            "env": env or {},
            # Make /v1/* reachable from the public internet on a vast.ai-assigned port.
            "runtype": "ssh",
        }
        raw = self._request("PUT", f"/asks/{offer_id}/", data=json.dumps(body))
        if not raw or not raw.get("success"):
            raise VastError(f"create_instance failed: {raw}")
        return raw["new_contract"]

    def list_instances(self) -> list[Instance]:
        raw = self._request("GET", "/instances/")
        return [Instance.from_api(i) for i in (raw.get("instances") or [])]

    def get_instance(self, instance_id: int) -> Instance:
        raw = self._request("GET", f"/instances/{instance_id}/")
        # API may return either {"instances": [...]} or the bare object.
        if isinstance(raw, dict) and "instances" in raw:
            for inst in raw["instances"]:
                if inst.get("id") == instance_id:
                    return Instance.from_api(inst)
            raise VastError(f"instance {instance_id} not found")
        return Instance.from_api(raw)

    def destroy_instance(self, instance_id: int) -> None:
        self._request("DELETE", f"/instances/{instance_id}/")


# ----- Helpers shared by the up/down/status commands ---------------------------


def load_config(path: str | Path = "config.yaml") -> dict:
    p = Path(path)
    if not p.exists():
        raise VastError(f"config file not found: {p} — copy config.example.yaml")
    raw = p.read_text()
    raw = string.Template(raw).safe_substitute(os.environ)
    return yaml.safe_load(raw)


def build_offer_query(cfg: dict) -> dict[str, Any]:
    """Translate our config block into vast.ai's filter syntax."""
    gpu = cfg["gpu"]
    host = cfg["host"]
    q: dict[str, Any] = {
        "rentable": {"eq": True},
        "verified": {"eq": True} if host.get("verified_only", True) else {},
        "num_gpus": {"eq": gpu.get("count", 1)},
        "gpu_total_ram": {"gte": gpu.get("min_vram_gb", 24) * 1024},
        "dph_total": {"lte": host.get("max_dph", 1.0)},
        "reliability2": {"gte": host.get("min_reliability", 0.95)},
        "disk_space": {"gte": host.get("min_disk_gb", 60)},
        "inet_down": {"gte": host.get("min_inet_down_mbps", 200)},
        "order": [["dph_total", "asc"]],
    }
    allowed = gpu.get("allowed")
    if allowed:
        q["gpu_name"] = {"in": allowed}
    return q


def render_onstart(cfg: dict, repo_root: Path) -> str:
    """Inline the cloud-init script + the env that drives it.

    vast.ai runs `onstart` after the rental boots. We pipe the bootstrap
    script through bash so the host doesn't need to fetch anything beyond the
    base image.
    """
    script_path = repo_root / cfg["bootstrap"]["script"]
    if not script_path.exists():
        raise VastError(f"bootstrap script not found: {script_path}")
    script = script_path.read_text()

    api_key = cfg["model"]["api_key"]
    if not api_key:
        raise VastError("model.api_key resolved to empty — set LLM_API_KEY in the env")

    env_lines = [
        f'export MODEL_NAME={shell_quote(cfg["model"]["name"])}',
        f'export LLM_API_KEY={shell_quote(api_key)}',
        f'export PORT={int(cfg["bootstrap"]["internal_port"])}',
        f'export GPU_MEM_FRAC={float(cfg["model"].get("gpu_mem_fraction", 0.92))}',
    ]
    if cfg["model"].get("hf_token"):
        env_lines.append(f'export HF_TOKEN={shell_quote(cfg["model"]["hf_token"])}')

    return "\n".join(env_lines) + "\n" + script


def shell_quote(s: str) -> str:
    if re.fullmatch(r"[A-Za-z0-9_./:@%+=-]+", s or ""):
        return s
    return "'" + s.replace("'", "'\"'\"'") + "'"


def repo_root_from(start: Path) -> Path:
    """Walk up to find the git root so paths in config are repo-relative."""
    p = start.resolve()
    while p != p.parent:
        if (p / ".git").exists():
            return p
        p = p.parent
    return start.resolve()


def fail(msg: str, code: int = 1) -> "NoReturn":
    print(f"error: {msg}", file=sys.stderr)
    sys.exit(code)


def wait_for_ports(client: VastClient, instance_id: int, internal_port: int, timeout_s: int = 600) -> Instance:
    """Poll until the instance has a public port mapped for `internal_port`."""
    deadline = time.time() + timeout_s
    last_status = ""
    while time.time() < deadline:
        inst = client.get_instance(instance_id)
        if inst.status != last_status:
            print(f"  status={inst.status}")
            last_status = inst.status
        if str(internal_port) in inst.exposed_ports and inst.public_ip:
            return inst
        time.sleep(5)
    raise VastError(f"timed out waiting for instance {instance_id} to expose port {internal_port}")
