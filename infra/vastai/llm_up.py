#!/usr/bin/env python3
"""Spin up an on-demand LLM endpoint on vast.ai.

Usage: VAST_API_KEY=… LLM_API_KEY=… python3 llm_up.py [--config config.yaml]

Steps:
  1. Search the marketplace for offers matching config.yaml.
  2. Pick the cheapest matching offer.
  3. Rent it; vast.ai provisions a CUDA-ready Ubuntu container.
  4. The onstart script (infra/cloud-init/llm-server.sh) installs vLLM and
     starts an OpenAI-compatible server.
  5. Wait for the public port to come up; print the endpoint URL.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from vastai_client import (
    VastClient,
    VastError,
    build_offer_query,
    fail,
    load_config,
    render_onstart,
    repo_root_from,
    wait_for_ports,
)

# vast.ai PyTorch CUDA base image — vLLM containers run inside Docker on top.
DEFAULT_IMAGE = "pytorch/pytorch:2.4.0-cuda12.4-cudnn9-runtime"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", default="config.yaml", help="path to config.yaml")
    parser.add_argument("--image", default=DEFAULT_IMAGE,
                        help="base Docker image for the vast.ai container")
    parser.add_argument("--dry-run", action="store_true",
                        help="search and print the top offer without renting it")
    args = parser.parse_args(argv)

    try:
        cfg = load_config(args.config)
        client = VastClient()
        repo_root = repo_root_from(Path(args.config).parent)
        onstart = render_onstart(cfg, repo_root)
        query = build_offer_query(cfg)

        print(f"Searching offers for {cfg['gpu'].get('allowed') or 'any GPU'} "
              f"(<= ${cfg['host'].get('max_dph')}/hr, "
              f">= {cfg['host'].get('min_reliability')} reliability)…")
        offers = client.search_offers(query)
        if not offers:
            fail("no offers matched the filter — relax max_dph, gpu list, or reliability")

        # `order` in the query already sorts by price ascending.
        chosen = offers[0]
        print(f"Top match: offer #{chosen.id}: {chosen.num_gpus}x {chosen.gpu_name} "
              f"@ ${chosen.dph_total:.3f}/hr, "
              f"{chosen.disk_gb:.0f}GB disk, "
              f"{chosen.inet_down_mbps:.0f}Mbps down, "
              f"reliability={chosen.reliability:.3f}")

        if args.dry_run:
            print(json.dumps([o.__dict__ for o in offers[:5]], indent=2))
            return 0

        port = int(cfg["bootstrap"]["internal_port"])
        print(f"Renting offer #{chosen.id}…")
        instance_id = client.create_instance(
            offer_id=chosen.id,
            image=args.image,
            onstart_cmd=onstart,
            label=cfg.get("tag", "kube-watcher-llm"),
            disk_gb=int(cfg["host"].get("min_disk_gb", 60)),
        )
        print(f"Instance #{instance_id} created. Waiting for port :{port} to be exposed…")
        inst = wait_for_ports(client, instance_id, port, timeout_s=900)

        public_port = inst.exposed_ports[str(port)]
        endpoint = f"http://{inst.public_ip}:{public_port}"
        print()
        print("──────────────────────────────────────────────────────────────")
        print(f"  Instance ID  : {instance_id}")
        print(f"  GPU          : {inst.gpu_name}")
        print(f"  Cost         : ${inst.dph_total:.3f}/hr")
        print(f"  Endpoint     : {endpoint}")
        print(f"  Auth header  : Authorization: Bearer $LLM_API_KEY")
        print(f"  Tear down    : python3 llm_down.py {instance_id}")
        print()
        print("  NOTE: the model still has to download (multi-GB). vLLM may take")
        print("  several more minutes before /v1/chat/completions answers.")
        print("──────────────────────────────────────────────────────────────")
        return 0

    except VastError as e:
        fail(str(e))


if __name__ == "__main__":
    sys.exit(main())
