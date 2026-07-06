#!/usr/bin/env python3
"""install.py — render a flavored copy of the orchestrate skill.

The myagent installer invokes this as:

    python3 install.py --dest <dir>            # options JSON on stdin

It copies the skill (minus the template-only files) into <dir> and writes
scripts/flavor.resolved.json, which orch.py seeds its config from at init. This
is what turns the abstract flavor choices (two ordered model tiers, codex on/off,
etc.) into concrete orch.py config.

Stdlib only.
"""
import argparse
import json
import os
import shutil
import sys

# Files that describe the flavor template itself — never copied into a render.
TEMPLATE_ONLY = {"install.py", "flavor.json"}
IGNORE = shutil.ignore_patterns("__pycache__", "*.pyc", ".DS_Store")


def resolve_config(opts):
    """Map flavor option values onto orch.py's config shape."""
    primary = opts.get("models.primary") or ["claude-opus-4-8"]
    secondary = opts.get("models.secondary") or ["claude-fable-5"]
    p0, s0 = primary[0], secondary[0]
    cfg = {
        # per-role models: primary tier drives everything except the architect
        "models": {
            "manager": p0,
            "builder": p0,
            "spec_preflight": p0,
            "code_preflight": p0,
            "architect": s0,
        },
        # full ordered preference lists, for fallback when a model is unavailable
        "models_primary": primary,
        "models_secondary": secondary,
        "use_codex": bool(opts.get("use_codex", True)),
        "codex_model": opts.get("codex_model", "") if opts.get("use_codex", True) else "",
        "test_cmd": opts.get("test_cmd", ""),
        "primary_branch": opts.get("primary_branch", "main"),
        "auto_advance_to_build": bool(opts.get("auto_advance_to_build", False)),
    }
    return cfg


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dest", required=True)
    args = ap.parse_args()

    raw = sys.stdin.read().strip()
    opts = json.loads(raw) if raw else {}

    src = os.path.dirname(os.path.abspath(__file__))
    os.makedirs(args.dest, exist_ok=True)

    for name in sorted(os.listdir(src)):
        if name in TEMPLATE_ONLY or name == "__pycache__" or name.startswith("."):
            continue
        s = os.path.join(src, name)
        d = os.path.join(args.dest, name)
        if os.path.isdir(s):
            shutil.copytree(s, d, ignore=IGNORE, dirs_exist_ok=True)
        else:
            shutil.copy2(s, d)

    scripts_dir = os.path.join(args.dest, "scripts")
    os.makedirs(scripts_dir, exist_ok=True)
    with open(os.path.join(scripts_dir, "flavor.resolved.json"), "w") as f:
        json.dump(resolve_config(opts), f, indent=2)
        f.write("\n")

    print(f"rendered orchestrate flavor into {args.dest}")


if __name__ == "__main__":
    main()
