#!/usr/bin/env python3
"""
orch.py — persistent state for the /orchestrate workflow.

The Manager (main Claude session) calls this to create, list, and update
workstreams. State lives in <root>/.orchestrate/ so it survives session
restarts and Claude-account switches (you resume by re-running /orchestrate).

Stdlib only. No dependencies.

Usage:
  orch.py init
  orch.py new "<title>"                 -> creates a workstream, prints its id
  orch.py list                          -> in-flight workstreams (not done/archived)
  orch.py list --all                    -> everything
  orch.py show <id>
  orch.py set <id> <field> <value>      -> field in: phase status round title branch
  orch.py round <id> [+1]               -> bump or print the current round
  orch.py log <id> "<message>"
  orch.py path <id> [dir|spec|reviews|notes]
  orch.py config [key] [value]          -> read/update .orchestrate/config.json

Phases:   spec, spec_review, awaiting_approval, build, build_review,
          integrate, done, archived, blocked
Statuses: in_progress, waiting_user, blocked, done

Field/phase values are not strictly validated so the Manager can adapt, but
these are the canonical ones the SKILL.md relies on.
"""
import json
import os
import re
import sys
import time
from datetime import datetime, timezone

ROOT = os.environ.get("ORCH_ROOT", os.getcwd())
BASE = os.path.join(ROOT, ".orchestrate")
STATE = os.path.join(BASE, "state.json")
STATUS_MD = os.path.join(BASE, "STATUS.md")

DEFAULT_CONFIG = {
    "auto_advance_to_build": False,   # False => stop at awaiting_approval gate
    "auto_advance_to_integrate": True,
    "codex_cmd": "codex exec --full-auto -s read-only",
    "codex_model": "",                # e.g. "gpt-5-codex-max"; blank => codex default
    "test_cmd": "",                   # e.g. "npm test" or "pytest -q"; blank => skip
    "primary_branch": "main",
    "memory_file": ".orchestrate/memory.md",
    "backlog_file": ".orchestrate/backlog.md",
    "tracker_file": ".orchestrate/STATUS.md",
    "models": {
        "manager": "opus",
        "architect": "claude-fable-5",
        "spec_preflight": "opus",
        "builder": "opus",
        "code_preflight": "opus",
    },
}

ACTIVE_PHASES = {"spec", "spec_review", "awaiting_approval",
                 "build", "build_review", "integrate", "blocked"}


def now():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def slug(title):
    s = re.sub(r"[^a-z0-9]+", "-", title.lower()).strip("-")
    return s[:40] or "workstream"


def load():
    if not os.path.exists(STATE):
        return {"seq": 0, "workstreams": {}, "config": dict(DEFAULT_CONFIG)}
    with open(STATE) as f:
        data = json.load(f)
    # backfill config defaults so upgrades don't lose keys
    cfg = data.setdefault("config", {})
    for k, v in DEFAULT_CONFIG.items():
        cfg.setdefault(k, v)
    return data


def save(data):
    os.makedirs(BASE, exist_ok=True)
    tmp = STATE + ".tmp"
    with open(tmp, "w") as f:
        json.dump(data, f, indent=2)
    os.replace(tmp, STATE)
    render_status(data)


def render_status(data):
    lines = ["# Orchestrate — workstream status", "",
             f"_Updated {now()}_", ""]
    ws = data["workstreams"]
    active = [w for w in ws.values() if w["phase"] in ACTIVE_PHASES]
    done = [w for w in ws.values() if w["phase"] in ("done", "archived")]

    def row(w):
        return (f"- **{w['id']}** — {w['title']}  \n"
                f"  phase: `{w['phase']}` · status: `{w['status']}` · "
                f"round: {w['round']} · updated: {w['updated']}")

    lines.append("## In flight")
    lines += [row(w) for w in sorted(active, key=lambda x: x["id"])] or ["_none_"]
    lines += ["", "## Completed"]
    lines += [row(w) for w in sorted(done, key=lambda x: x["id"])] or ["_none_"]
    lines.append("")
    with open(STATUS_MD, "w") as f:
        f.write("\n".join(lines))


def ws_dir(wid):
    return os.path.join(BASE, "workstreams", wid)


def get(data, wid):
    w = data["workstreams"].get(wid)
    if not w:
        sys.exit(f"error: no workstream '{wid}'. Run: orch.py list")
    return w


# ---- commands ---------------------------------------------------------------

def cmd_init(_args):
    data = load()
    os.makedirs(os.path.join(BASE, "workstreams"), exist_ok=True)
    for fname in (data["config"]["memory_file"], data["config"]["backlog_file"]):
        p = os.path.join(ROOT, fname)
        if not os.path.exists(p):
            os.makedirs(os.path.dirname(p), exist_ok=True)
            title = "Memory" if "memory" in fname else "Backlog"
            open(p, "w").write(f"# {title}\n\n")
    save(data)
    print(f"initialized {BASE}")


def cmd_new(args):
    if not args:
        sys.exit('error: orch.py new "<title>"')
    title = args[0]
    data = load()
    data["seq"] += 1
    wid = f"ws-{data['seq']:03d}-{slug(title)}"
    d = ws_dir(wid)
    os.makedirs(os.path.join(d, "reviews"), exist_ok=True)
    w = {"id": wid, "title": title, "phase": "spec", "status": "in_progress",
         "round": 0, "branch": "", "created": now(), "updated": now(),
         "log": [{"t": now(), "m": "created"}]}
    data["workstreams"][wid] = w
    for f in ("spec.md", "notes.md"):
        p = os.path.join(d, f)
        if not os.path.exists(p):
            open(p, "w").write(f"# {title} — {f.split('.')[0]}\n\n")
    save(data)
    print(wid)


def cmd_list(args):
    data = load()
    show_all = "--all" in args
    ws = data["workstreams"].values()
    rows = [w for w in ws if show_all or w["phase"] in ACTIVE_PHASES]
    if not rows:
        print("(no in-flight workstreams — start a new one)")
        return
    for w in sorted(rows, key=lambda x: x["id"]):
        print(f"{w['id']:<34} phase={w['phase']:<17} "
              f"status={w['status']:<12} round={w['round']}  {w['title']}")


def cmd_show(args):
    data = load()
    w = get(data, args[0])
    print(json.dumps(w, indent=2))
    print("\nfiles:")
    print("  dir:    ", ws_dir(w["id"]))
    print("  spec:   ", os.path.join(ws_dir(w["id"]), "spec.md"))
    print("  reviews:", os.path.join(ws_dir(w["id"]), "reviews"))


def cmd_set(args):
    if len(args) < 3:
        sys.exit("error: orch.py set <id> <field> <value>")
    wid, field, value = args[0], args[1], " ".join(args[2:])
    data = load()
    w = get(data, wid)
    if field == "round":
        value = int(value)
    w[field] = value
    w["updated"] = now()
    w.setdefault("log", []).append({"t": now(), "m": f"set {field}={value}"})
    save(data)
    print(f"{wid}: {field} -> {value}")


def cmd_round(args):
    data = load()
    w = get(data, args[0])
    if len(args) > 1 and args[1] in ("+1", "++"):
        w["round"] += 1
        w["updated"] = now()
        w.setdefault("log", []).append({"t": now(), "m": f"round -> {w['round']}"})
        save(data)
    print(w["round"])


def cmd_log(args):
    if len(args) < 2:
        sys.exit('error: orch.py log <id> "<message>"')
    wid, msg = args[0], " ".join(args[1:])
    data = load()
    w = get(data, wid)
    w.setdefault("log", []).append({"t": now(), "m": msg})
    w["updated"] = now()
    save(data)
    print("logged")


def cmd_path(args):
    data = load()
    w = get(data, args[0])
    which = args[1] if len(args) > 1 else "dir"
    d = ws_dir(w["id"])
    print({"dir": d,
           "spec": os.path.join(d, "spec.md"),
           "notes": os.path.join(d, "notes.md"),
           "reviews": os.path.join(d, "reviews")}.get(which, d))


def cmd_config(args):
    data = load()
    cfg = data["config"]
    if not args:
        print(json.dumps(cfg, indent=2))
        return
    key = args[0]
    if len(args) == 1:
        print(json.dumps(cfg.get(key), indent=2))
        return
    val = " ".join(args[1:])
    if val.lower() in ("true", "false"):
        val = val.lower() == "true"
    cfg[key] = val
    save(data)
    print(f"config {key} -> {val}")


COMMANDS = {
    "init": cmd_init, "new": cmd_new, "list": cmd_list, "show": cmd_show,
    "set": cmd_set, "round": cmd_round, "log": cmd_log, "path": cmd_path,
    "config": cmd_config,
}


def main():
    if len(sys.argv) < 2 or sys.argv[1] not in COMMANDS:
        print(__doc__)
        sys.exit(0 if len(sys.argv) < 2 else 2)
    COMMANDS[sys.argv[1]](sys.argv[2:])


if __name__ == "__main__":
    main()
