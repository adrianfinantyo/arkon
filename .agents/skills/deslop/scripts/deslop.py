#!/usr/bin/env python3
"""
deslop: Scan text, UI copy, code, and git changes for AI slop.
Enforces rules defined in .agents/rules/deslop.md.
"""

import sys
import os
import re
import subprocess
import argparse

# ---------------------------------------------------------------------------
# Paths that contain banned words as examples (rules, skill, and scanner itself)
# ---------------------------------------------------------------------------
DESLOP_META_MARKERS = [
    ".agents/skills/deslop/",
    ".agents/rules/deslop",
    "agents/skills/deslop/",
    "agents/rules/deslop",
]


def _is_deslop_meta(filepath):
    """Return True if filepath is part of the deslop rules/skill definition or scanner itself."""
    norm = filepath.replace("\\", "/")
    if os.path.basename(norm) in ("deslop.py", "deslop.md"):
        return True
    for marker in DESLOP_META_MARKERS:
        if marker in norm:
            return True
    return False


# ---------------------------------------------------------------------------
# Pattern lists
# ---------------------------------------------------------------------------

EM_DASH_PATTERNS = [
    (r"\u2014", "Em-dash detected. Use a period, comma, colon, or parentheses instead."),
    (r"\s+\u2013\s+", "Spaced en-dash used as em-dash. Use a period, comma, or colon instead."),
    (r"\s+--\s+", "Spaced double-hyphen used as em-dash. Use a period, comma, or colon instead."),
]

BUZZWORD_PATTERNS = [
    (r"\bdelv(e|es|ed|ing)\b", "delve -> explore, examine, look into"),
    (r"\bleverag(e|es|ed|ing)\b", "leverage -> use, rely on, apply"),
    (r"\btestament\b", "testament -> proof, evidence, shows, reflects"),
    (r"\btapestr(y|ies)\b", "tapestry -> mix, system, collection, variety"),
    (r"\bseamless(ly)?\b", "seamless -> smooth, automatic, direct"),
    (r"\bbeacon\b", "beacon -> model, example, guide"),
    (r"\bgame-?changer\b", "game-changer -> major improvement, significant shift"),
    (r"\bmeticulous(ly)?\b", "meticulous -> careful, thorough, detailed"),
    (r"\bcrucial(ly)?\b", "crucial -> key, important, needed"),
    (r"\bpivotal\b", "pivotal -> key, vital, important"),
    (r"\bparamount\b", "paramount -> top priority, vital, essential"),
    (r"\bmultifaceted\b", "multifaceted -> varied, broad, comprehensive"),
    (r"\bholistic(ally)?\b", "holistic -> complete, broad, comprehensive"),
    (r"\bfoster(s|ed|ing)?\b", "foster -> support, build, encourage, help"),
    (r"\bempower(s|ed|ing|ment)?\b", "empower -> enable, allow, help"),
    (r"\brevolutioniz(e|es|ed|ing)\b", "revolutionize -> modernize, overhaul, update"),
    (r"\bcutting-?edge\b", "cutting-edge -> modern, current, updated"),
    (r"\bdeep\s+dive\b", "deep dive -> thorough look, detailed analysis"),
    (r"\bdive\s+deep\b", "dive deep -> look closely, analyze thoroughly"),
    (r"\bplethora\b", "plethora -> many, numerous"),
    (r"\bmyriad\b", "myriad -> many, numerous, several"),
    (r"\bnestled\b", "nestled -> located, placed"),
    (r"\bunravel(s|ed|ing)?\b", "unravel -> explain, clarify, resolve"),
    (r"\butiliz(e|es|ed|ing|ation)\b", "utilize -> use"),
    (r"\brobust(ness|ly)?\b", "robust -> strong, reliable, solid"),
    (r"\bfacilitat(e|es|ed|ing)\b", "facilitate -> enable, help, allow"),
    (r"\bstreamlin(e|es|ed|ing)\b", "streamline -> simplify, reduce, speed up"),
    (r"\baforementioned\b", "aforementioned -> (refer to it by name instead)"),
]

SUGAR_COATING_PATTERNS = [
    (r"\bCertainly[!.,]", "Sycophantic throat-clearing. State the answer directly."),
    (r"\bI('d| would) be (thrilled|happy|delighted|glad) to\b", "Sycophantic preamble. Skip the pleasantry."),
    (r"\bGreat question[!.,]", "Performative praise. Answer directly."),
    (r"\bIn today's fast-paced\b", "Cliche throat-clearing. State the problem directly."),
    (r"\bIt is (important|crucial|essential) to (note|remember) that\b", "Wordy filler. State the fact directly."),
    (r"\bAs an AI (language model|assistant)\b", "AI disclosure boilerplate. State the facts directly."),
    (r"^(In conclusion|In summary|Overall, by doing so)[,:]?", "Formulaic conclusion. Remove the wrap-up."),
    (r"\bit('s| is) worth (noting|mentioning) that\b", "Filler preamble. State the fact directly."),
]

CODE_COMMENT_SLOP_PATTERNS = [
    (r"(//|#)\s*(increment|decrement)\s+\w+", "Redundant comment restating obvious operation."),
    (r"(//|#)\s*return\s+(the\s+)?result", "Redundant comment restating return statement."),
    (r"(//|#)\s*check if\s+\w+\s+is\s+(null|nil|empty|none)", "Redundant comment restating conditional."),
    (r"(//|#)\s*import\s+(libraries|packages|modules)", "Redundant comment restating import block."),
    (r"(//|#)\s*loop (through|over)\s+\w+", "Redundant comment restating iteration."),
]

EXCLUDED_EXTENSIONS = {
    ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".webp",
    ".lock", ".sum", ".bin", ".tar", ".gz", ".zip", ".pdf",
    ".woff", ".woff2", ".ttf", ".eot", ".mp4", ".mp3",
}

EXCLUDED_PATHS = [
    ".git/", "node_modules/", "vendor/", "dist/", "build/",
]


def is_text_file(filename):
    _, ext = os.path.splitext(filename)
    if ext.lower() in EXCLUDED_EXTENSIONS:
        return False
    for excluded in EXCLUDED_PATHS:
        if excluded in filename:
            return False
    return True


def check_line(text, is_code=False):
    violations = []
    for pat, msg in EM_DASH_PATTERNS:
        if re.search(pat, text):
            violations.append(("Punctuation", msg))

    for pat, msg in BUZZWORD_PATTERNS:
        match = re.search(pat, text, re.IGNORECASE)
        if match:
            violations.append(("Buzzword", f"Found '{match.group(0)}': {msg}"))

    for pat, msg in SUGAR_COATING_PATTERNS:
        match = re.search(pat, text, re.IGNORECASE)
        if match:
            violations.append(("Sugar-coating", f"Found '{match.group(0)}': {msg}"))

    if is_code:
        for pat, msg in CODE_COMMENT_SLOP_PATTERNS:
            match = re.search(pat, text, re.IGNORECASE)
            if match:
                violations.append(("Code Slop", f"Redundant comment: {msg}"))

    return violations


def check_file_content(filepath):
    if _is_deslop_meta(filepath):
        return []

    results = []
    is_code = filepath.endswith((".go", ".ts", ".js", ".py", ".rs", ".java", ".c", ".cpp"))
    try:
        with open(filepath, "r", encoding="utf-8", errors="ignore") as f:
            for idx, line in enumerate(f, 1):
                v = check_line(line, is_code=is_code)
                for cat, msg in v:
                    results.append((filepath, idx, line.strip(), cat, msg))
    except Exception as e:
        print(f"Error reading {filepath}: {e}", file=sys.stderr)
    return results


def _check_diff_output(diff_text):
    """Parse unified diff output and check added lines for slop."""
    results = []
    current_file = None
    line_num = 0

    for line in diff_text.splitlines():
        if line.startswith("+++ b/"):
            current_file = line[6:]
            continue
        if line.startswith("@@"):
            m = re.search(r"\+(\d+)(?:,(\d+))?", line)
            if m:
                line_num = int(m.group(1))
            continue
        if line.startswith("+") and not line.startswith("+++"):
            added_content = line[1:]
            if current_file and is_text_file(current_file) and not _is_deslop_meta(current_file):
                is_code = current_file.endswith((".go", ".ts", ".js", ".py", ".rs", ".java", ".c", ".cpp"))
                v = check_line(added_content, is_code=is_code)
                for cat, msg in v:
                    results.append((current_file, line_num, added_content.strip(), cat, msg))
            line_num += 1

    return results


def check_staged_diff():
    try:
        proc = subprocess.run(
            ["git", "diff", "--cached", "--unified=0"],
            capture_output=True, text=True, check=True,
        )
    except subprocess.CalledProcessError as e:
        print(f"Error executing git diff: {e}", file=sys.stderr)
        return []
    return _check_diff_output(proc.stdout)


def check_commit_diff(commit_ref):
    try:
        proc = subprocess.run(
            ["git", "show", "--unified=0", commit_ref],
            capture_output=True, text=True, check=True,
        )
    except subprocess.CalledProcessError as e:
        print(f"Error executing git show for {commit_ref}: {e}", file=sys.stderr)
        return []
    return _check_diff_output(proc.stdout)


def get_all_tracked_files():
    try:
        proc = subprocess.run(["git", "ls-files"], capture_output=True, text=True, check=True)
        return [f for f in proc.stdout.splitlines() if is_text_file(f)]
    except subprocess.CalledProcessError:
        return []


def print_report(violations, quiet=False):
    if not violations:
        if not quiet:
            print("\u2713 No AI slop detected. Content is clean, direct, and human-to-human.")
        return 0

    print(f"\n\u274c Found {len(violations)} instance(s) of AI slop:")
    print("=" * 70)
    for filepath, line_num, content, category, suggestion in violations:
        print(f"  {filepath}:{line_num}")
        print(f"  [{category}] {suggestion}")
        if not quiet:
            print(f"  > {content}")
        print("-" * 70)

    if not quiet:
        print("\nFix by replacing flagged words with plain alternatives.")
        print("See .agents/rules/deslop.md for the full replacement table.\n")
    return 1


def main():
    parser = argparse.ArgumentParser(description="Scan for AI slop in text, code, and git changes.")
    parser.add_argument("--files", nargs="+", help="Specific files to scan")
    parser.add_argument("--commit", help="Commit SHA or ref to inspect")
    parser.add_argument("--staged", action="store_true", help="Inspect staged git changes")
    parser.add_argument("--all", action="store_true", help="Scan all git-tracked files")
    parser.add_argument("--force", action="store_true", help="Skip confirmation when scanning all files")
    parser.add_argument("--quiet", "-q", action="store_true", help="Minimal output (for pre-commit hooks)")

    args = parser.parse_args()

    if args.staged:
        violations = check_staged_diff()
        sys.exit(print_report(violations, quiet=args.quiet))

    if args.commit:
        violations = check_commit_diff(args.commit)
        sys.exit(print_report(violations, quiet=args.quiet))

    if args.files:
        violations = []
        for f in args.files:
            if os.path.exists(f) and is_text_file(f):
                violations.extend(check_file_content(f))
        sys.exit(print_report(violations, quiet=args.quiet))

    if args.all:
        if not args.force:
            response = input("Scan all tracked files? [y/N]: ").strip().lower()
            if response not in ("y", "yes"):
                print("Aborted.")
                sys.exit(0)
        files = get_all_tracked_files()
        violations = []
        for f in files:
            violations.extend(check_file_content(f))
        sys.exit(print_report(violations, quiet=args.quiet))

    print("No input specified. Use --files, --commit, --staged, or --all.")
    sys.exit(1)


if __name__ == "__main__":
    main()
