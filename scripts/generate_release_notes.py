#!/usr/bin/env python3
import json
import os
import subprocess
import sys
import urllib.request

REPO = "Octrafic/octrafic-cli"
MODEL = "mistralai/mistral-large-2512"
OPENROUTER_API = "https://openrouter.ai/api/v1/chat/completions"

OWNER = "mikolajbadyl"

SYSTEM_PROMPT = """You generate release notes for a CLI tool called Octrafic.

Given a list of commits (each with a message and author), write release notes grouped into these sections:
- Features (new functionality)
- Bug Fixes (fixes to existing functionality)

Rules:
- Only include sections that have content
- Each item is a short, clear sentence a user can understand — no technical jargon, no commit prefixes like feat:/fix:/chore:
- Skip purely internal changes: docs, CI, linting, refactoring, dependency bumps
- Keep each item concise but descriptive enough to be useful
- Use GitHub Markdown bullet lists under each section heading
- If a commit was made by an external contributor (author field is not the owner), append their GitHub username at the end of the item like: (@username)
- Output ONLY the Markdown — no preamble, no intro sentence, no explanation

Example output:
## Features

- Add support for Google Gemini models
- Normalize file status paths relative to instance directory (@shantur)

## Bug Fixes

- Fix crash when response body is empty
"""


def run(cmd: list[str]) -> str:
    result = subprocess.run(cmd, capture_output=True, text=True, check=True)
    return result.stdout.strip()


def get_all_releases() -> list[dict]:
    out = run(["gh", "release", "list", "--repo", REPO, "--limit", "50", "--json", "tagName,createdAt"])
    return json.loads(out)


def get_commits_between(prev_tag: str | None, tag: str) -> list[dict]:
    if prev_tag:
        ref = f"{prev_tag}...{tag}"
        out = run(["gh", "api", f"repos/{REPO}/compare/{ref}", "--jq", ".commits[] | {message: .commit.message, author: .author.login}"])
    else:
        out = run(["gh", "api", f"repos/{REPO}/commits?sha={tag}&per_page=30", "--jq", ".[] | {message: .commit.message, author: .author.login}"])
    commits = []
    for line in out.splitlines():
        line = line.strip()
        if line:
            try:
                commits.append(json.loads(line))
            except json.JSONDecodeError:
                pass
    return commits


def generate_notes(commits: list[dict]) -> str:
    api_key = os.environ.get("OPENROUTER_API_KEY")
    if not api_key:
        raise RuntimeError("OPENROUTER_API_KEY environment variable not set")

    commit_lines = []
    for c in commits:
        author = c.get("author") or "unknown"
        label = f"[external: @{author}]" if author != OWNER else "[owner]"
        msg = c.get("message", "").splitlines()[0]
        commit_lines.append(f"- {msg} {label}")

    payload = json.dumps({
        "model": MODEL,
        "messages": [
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": f"Owner: {OWNER}\nCommits:\n" + "\n".join(commit_lines)},
        ],
    }).encode()

    req = urllib.request.Request(
        OPENROUTER_API,
        data=payload,
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        },
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read())

    return data["choices"][0]["message"]["content"].strip()


def update_release(tag: str, notes: str) -> None:
    run(["gh", "release", "edit", tag, "--repo", REPO, "--notes", notes])


def process_release(tag: str, prev_tag: str | None) -> None:
    print(f"Processing {tag}...")
    commits = get_commits_between(prev_tag, tag)
    if not commits:
        print(f"  No commits found, skipping.")
        return
    notes = generate_notes(commits)
    update_release(tag, notes)
    print(f"  Updated.\n{notes}\n")


def main() -> None:
    if len(sys.argv) < 2:
        print("Usage: python scripts/generate_release_notes.py <version>", file=sys.stderr)
        print("Example: python scripts/generate_release_notes.py v0.4.2", file=sys.stderr)
        sys.exit(1)

    target = sys.argv[1]

    releases = get_all_releases()
    releases_sorted = sorted(releases, key=lambda r: r["createdAt"])
    tags = [r["tagName"] for r in releases_sorted]

    if target not in tags:
        print(f"Release {target} not found.", file=sys.stderr)
        sys.exit(1)

    idx = tags.index(target)
    prev_tag = tags[idx - 1] if idx > 0 else None
    process_release(target, prev_tag)


if __name__ == "__main__":
    main()
