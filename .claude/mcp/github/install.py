#!/usr/bin/env python3
"""install.py — render a GitHub MCP server definition from flavor choices.

The myagent installer runs this for a flavored MCP template:

    python3 install.py --dest <dir>        # options JSON on stdin

It writes <dir>/server.json — the object as it appears inside "mcpServers" —
which the installer then merges into the target's MCP config. The token is a
${ENV_VAR} reference, never a literal secret.

Stdlib only.
"""
import argparse
import json
import os
import sys


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dest", required=True)
    args = ap.parse_args()

    raw = sys.stdin.read().strip()
    opts = json.loads(raw) if raw else {}

    token_env = opts.get("token_env", "GITHUB_TOKEN")
    server_args = ["-y", "@modelcontextprotocol/server-github"]
    if opts.get("readonly", True):
        server_args.append("--read-only")

    server = {
        "command": "npx",
        "args": server_args,
        "env": {"GITHUB_PERSONAL_ACCESS_TOKEN": "${" + token_env + "}"},
    }

    os.makedirs(args.dest, exist_ok=True)
    with open(os.path.join(args.dest, "server.json"), "w") as f:
        json.dump(server, f, indent=2)
        f.write("\n")
    print("rendered github MCP server.json")


if __name__ == "__main__":
    main()
