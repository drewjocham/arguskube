#!/usr/bin/env python3
"""Destroy vast.ai LLM instances.

Usage:
  python3 llm_down.py             # destroy every instance tagged kube-watcher-llm
  python3 llm_down.py 12345 6789  # destroy specific instance ids
  python3 llm_down.py --all       # destroy ALL of your vast.ai instances (careful)
"""

from __future__ import annotations

import argparse
import sys

from vastai_client import VastClient, VastError, fail, load_config


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("ids", nargs="*", type=int, help="explicit instance ids")
    parser.add_argument("--config", default="config.yaml")
    parser.add_argument("--all", action="store_true",
                        help="destroy every instance, including untagged ones")
    parser.add_argument("--yes", action="store_true", help="skip confirmation")
    args = parser.parse_args(argv)

    try:
        client = VastClient()
        targets: list[int] = list(args.ids)

        if not targets:
            instances = client.list_instances()
            if args.all:
                targets = [i.id for i in instances]
            else:
                tag = "kube-watcher-llm"
                try:
                    cfg = load_config(args.config)
                    tag = cfg.get("tag", tag)
                except VastError:
                    pass  # config is optional for tear-down
                targets = [i.id for i in instances if i.label == tag]

        if not targets:
            print("nothing to destroy.")
            return 0

        print(f"Will destroy {len(targets)} instance(s): {targets}")
        if not args.yes:
            resp = input("Type 'destroy' to confirm: ").strip().lower()
            if resp != "destroy":
                print("aborted.")
                return 1

        for inst_id in targets:
            try:
                client.destroy_instance(inst_id)
                print(f"  ✓ destroyed #{inst_id}")
            except VastError as e:
                print(f"  ✗ #{inst_id}: {e}", file=sys.stderr)
        return 0

    except VastError as e:
        fail(str(e))


if __name__ == "__main__":
    sys.exit(main())
