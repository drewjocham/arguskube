#!/usr/bin/env python3
"""List vast.ai LLM instances and print their endpoints.

Usage: python3 llm_status.py [--config config.yaml] [--all]
"""

from __future__ import annotations

import argparse
import sys

from vastai_client import VastClient, VastError, fail, load_config


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", default="config.yaml")
    parser.add_argument("--all", action="store_true",
                        help="show every instance, not just our tagged ones")
    args = parser.parse_args(argv)

    try:
        client = VastClient()
        instances = client.list_instances()
        tag = "argus-llm"
        try:
            cfg = load_config(args.config)
            tag = cfg.get("tag", tag)
        except VastError:
            pass

        if not args.all:
            instances = [i for i in instances if i.label == tag]

        if not instances:
            print(f"no instances found ({'all' if args.all else f'tag={tag}'}).")
            return 0

        # Header
        print(f"{'ID':<10} {'STATUS':<14} {'GPU':<14} {'$/hr':<7} {'ENDPOINT'}")
        for inst in instances:
            endpoint = "—"
            for internal, public in inst.exposed_ports.items():
                if internal == "8000" and inst.public_ip:
                    endpoint = f"http://{inst.public_ip}:{public}"
                    break
            print(f"{inst.id:<10} {inst.status:<14} {inst.gpu_name:<14} "
                  f"{inst.dph_total:<7.3f} {endpoint}")
        return 0

    except VastError as e:
        fail(str(e))


if __name__ == "__main__":
    sys.exit(main())
