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
    """Map flavor option values onto orch.py's config shape.

    Config is organized by capability TIER, not by named role, so one knob covers
    everyone at that level:
      - staff (fable) — Manager + Architect: long-running, unsupervised, taste-heavy.
      - senior (opus) — Builder + structure/spec-conformance preflight.
      - junior (sonnet) — simple tasks + the codex-runner wrapper.
      - reviewer — cross-model correctness review, OUTSIDE the Claude hierarchy,
        via the external_agent CLI (`codex review`, or `agy`).
      - mechanical — computer-use + bulk work via that same external_agent CLI.
    staff/senior/junior are ordered fallback lists (fall UP when unavailable);
    reviewer/mechanical are single model values whose backend depends on
    external_agent ("codex" = gpt-5.5, "agy" = Gemini 3, "none" = no external gate).
    """
    staff = opts.get("models.staff") or ["claude-fable-5"]
    senior = opts.get("models.senior") or ["claude-opus-4-8"]
    junior = opts.get("models.junior") or ["claude-sonnet-5"]
    external_agent = opts.get("external_agent", "codex")

    # The reviewer/mechanical models depend on which external CLI is selected.
    if external_agent == "codex":
        reviewer = opts.get("models.reviewer", "gpt-5-codex-max")
        mechanical = opts.get("models.mechanical", "gpt-5-codex-max")
    elif external_agent == "agy":
        # agy pins a single model via --model; blank => agy's own default.
        # The same model backs both the reviewer gate and mechanical work.
        agy_model = opts.get("agy_model", "")
        reviewer = agy_model
        mechanical = agy_model
    elif external_agent == "none":
        reviewer = ""
        mechanical = ""
    else:
        raise ValueError(f"unsupported external_agent: {external_agent!r}")

    cfg = {
        "models": {
            "staff": staff[0],
            "senior": senior[0],
            "junior": junior[0],
            "reviewer": reviewer,
            "mechanical": mechanical,
        },
        # full ordered preference lists, for fallback (fall UP when unavailable)
        "models_staff": staff,
        "models_senior": senior,
        "models_junior": junior,
        "external_agent": external_agent,
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
