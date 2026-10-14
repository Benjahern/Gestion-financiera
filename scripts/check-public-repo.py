#!/usr/bin/env python3
"""Check public source and optional Git history without ever printing secrets."""
import argparse
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
SAFE_EMAIL_DOMAINS = {"example.com", "example.org", "example.invalid", "mail.machbank.cl"}
EMAIL = re.compile(rb"[A-Za-z0-9._%+-]+@([A-Za-z0-9.-]+\.[A-Za-z]{2,})")
PATTERNS = {
    "private key": rb"-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----",
    "Google OAuth credential": rb"GOCSPX-[A-Za-z0-9_-]{15,}",
    "Google OAuth client ID": rb"[0-9]{6,}-[a-z0-9]+\.apps\.googleusercontent\.com",
    "Google API key": rb"AIza[0-9A-Za-z_-]{35}",
    "GitHub token": rb"(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{40,})",
    "AWS credential": rb"(?:AKIA|ASIA)[A-Z0-9]{16}",
    "Slack token": rb"xox[baprs]-[A-Za-z0-9-]{20,}",
}
PRIVATE_SUFFIXES = {".xlsx", ".xls", ".xlsm", ".ods", ".csv", ".ofx", ".qfx", ".qif", ".numbers", ".dump", ".backup", ".pem", ".key", ".p12", ".pfx", ".sqlite", ".sqlite3", ".db"}

def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT)

def check(name, data):
    problems = []
    path = pathlib.PurePosixPath(name)
    if path.suffix.lower() in PRIVATE_SUFFIXES or (path.name.startswith(".env") and path.name != ".env.example") or re.match(r"(?:credentials|client_secret).*\.json$", path.name):
        problems.append("private file")
    for label, pattern in PATTERNS.items():
        if re.search(pattern, data):
            problems.append(label)
    for match in EMAIL.finditer(data):
        domain = match[1].decode("ascii").lower()
        if domain not in SAFE_EMAIL_DOMAINS and not domain.endswith((".example", ".invalid", ".test", ".example.com", ".example.org", ".users.noreply.github.com")) and domain != "users.noreply.github.com":
            problems.append("non-example email")
            break
    # Documentation/test fixtures are not live database credentials.
    if path.suffix not in {".md", ".py", ".yml", ".yaml"} and not name.endswith(("_test.go", ".env.example")):
        if re.search(rb"postgres(?:ql)?://[^\s:/]+:[^\s@$][^\s@]*@", data):
            problems.append("embedded database password")
    return problems

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--history", action="store_true")
    parser.add_argument("--staged", action="store_true")
    parser.add_argument("--export", type=pathlib.Path, help="copy only public candidate files for a second scanner")
    args = parser.parse_args()
    findings = []
    if args.staged:
        names = git("diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z").split(b"\0")
    else:
        names = git("ls-files", "--cached", "--others", "--exclude-standard", "-z").split(b"\0")
    for encoded in set(names):
        if not encoded:
            continue
        name = encoded.decode("utf-8", "surrogateescape")
        if args.staged:
            data = git("show", ":" + name)
        else:
            path = ROOT / name
            if not path.is_file():
                continue
            if path.is_symlink():
                findings.append((name, "symlink requires privacy review"))
                continue
            data = path.read_bytes()
        findings.extend((name, label) for label in check(name, data))
        if args.export:
            destination = args.export / name
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(data)
    if args.history:
        for line in git("rev-list", "--objects", "--all").decode().splitlines():
            object_id, _, name = line.partition(" ")
            if not name or git("cat-file", "-t", object_id).strip() != b"blob":
                continue
            data = git("cat-file", "blob", object_id)
            findings.extend((f"history:{object_id[:12]}:{name}", label) for label in check(name, data))
    for name, label in sorted(set(findings)):
        print(f"BLOCKED {label}: {name}")
    if findings:
        print("Values redacted. Do not publish until reviewed.")
        return 1
    print("PASS: public files" + (" and Git blobs" if args.history else "") + "; no configured private emails/provider credentials/financial files detected.")
    return 0

if __name__ == "__main__":
    sys.exit(main())
