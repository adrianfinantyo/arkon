#!/usr/bin/env python3
"""
deslop: Utility to detect and clean AI slop in text, UI copy, code, and git changes.
Enforces rules defined in .agents/rules/deslop.md.
"""

import sys
import os
import re
import subprocess
import argparse

# Common patterns for AI slop detection
EM_DASH_PATTERNS = [
    (r"—", "Em-dash ('—') detected. Use a period, comma, colon, or parentheses instead."),
    (r"\s+–\s+", "Spaced en-dash (' – ') used as an em-dash. Use a period, comma, or colon instead."),
    (r"\s+--\s+", "Spaced double hyphen (' -- ') used as an em-dash. Use a period, comma, or colon instead."),
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
    (r"\bpivotal\b", "pivotal -> key, vital, important"),
    (r"\bparamount\b", "paramount -> top priority, vital, essential"),
    (r"\bmultifaceted\b", "multifaceted -> varied, broad, comprehensive"),
    (r"\bholistic(ally)?\b", "holistic -> complete, broad, comprehensive"),
    (r"\bfoster(s|ed|ing)?\b", "foster -> support, build, encourage, help"),
    (r"\bempower(s|ed|ing)?\b", "empower -> enable, allow, help"),
    (r"\brevolutioniz(e|es|ed|ing)\b", "revolutionize -> modernize, overhaul, update"),
    (r"\bcutting-?edge\b", "cutting-edge -> modern, current, updated"),
    (r"\bdeep\s+dive\b", "deep dive -> thorough look, detailed analysis"),
    (r"\bdive\s+deep\b", "dive deep -> look closely, analyze thoroughly"),
    (r"\bplethora\b", "plethora -> many, numerous, abundance"),
    (r"\bmyriad\b", "myriad -> many, numerous, several"),
    (r"\bnestled\b", "nestled -> located, placed"),
    (r"\bunravel(s|ed|ing)?\b", "unravel -> explain, clarify, resolve"),
]

SUGAR_COATING_PATTERNS = [
    (r"\bCertainly[!.,]", "Sycophantic throat-clearing ('Certainly!'). State the answer directly."),
    (r"\bI('d| would) be (thrilled|happy|delighted) to\b", "Sycophantic preamble. Skip the pleasantry and proceed."),
    (r"\bGreat question[!.,]", "Performative praise ('Great question!'). Answer directly."),
    (r"\bIn today's fast-paced\b", "Cliche throat-clearing ('In today's fast-paced...'). State the problem directly."),
    (r"\bIt is (important|crucial|essential) to (note|remember) that\b", "Wordy filler. State the fact directly."),
    (r"\bAs an AI\b", "AI disclosure boilerplate. State the facts or limitations directly."),
    (r"^(In conclusion|In summary|Overall, by doing so)[,:]?", "Formulaic conclusion wrap-up. Remove unnecessary summary."),
]

CODE_COMMENT_SLOP_PATTERNS = [
    (r"(//|#)\s*(increment|decrement)\s+\w+", "Redundant comment restating obvious code operation."),
    (r"(//|#)\s*return\s+(the\s+)?result", "Redundant comment restating return statement."),
    (r"(//|#)\s*check if\s+\w+\s+is\s+(null|nil|empty|none)", "Redundant comment restating obvious conditional check."),
    (r"(//|#)\s*import\s+(libraries|packages|modules)", "Redundant comment restating import block."),
]

EXCLUDED_EXTENSIONS = {
    ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".webp",
    ".lock", ".sum", ".bin", ".tar", ".gz", ".zip", ".pdf"
}

EXCLUDED_PATHS = [
    ".git/", "node_modules/", "vendor/", "dist/", "build/", ".scratch/"
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
    # 1. Em-dash patterns
    for pat, msg in EM_DASH_PATTERNS:
        if re.search(pat, text):
            violations.append(("Punctuation", msg))

    # 2. Buzzwords
    for pat, msg in BUZZWORD_PATTERNS:
        match = re.search(pat, text, re.IGNORECASE)
        if match:
            # Avoid false positives in URLs or markdown anchors if applicable
            violations.append(("Buzzword", f"Found '{match.group(0)}': {msg}"))

    # 3. Sugar coating
    for pat, msg in SUGAR_COATING_PATTERNS:
        match = re.search(pat, text, re.IGNORECASE)
        if match:
            violations.append(("Sugar-coating", f"Found '{match.group(0)}': {msg}"))

    # 4. Code comments
    if is_code:
        for pat, msg in CODE_COMMENT_SLOP_PATTERNS:
            match = re.search(pat, text, re.IGNORECASE)
            if match:
                violations.append(("Code Slop", f"Found redundant comment: {msg}"))

    return violations


def check_file_content(filepath):
    # Do not scan the deslop checker scripts or rules files themselves
    basename = os.path.basename(filepath)
    if basename in ("deslop.py", "deslop.md"):
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


def check_staged_diff():
    results = []
    cmd = ["git", "diff", "--cached", "--unified=0"]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, check=True)
    except subprocess.CalledProcessError as e:
        print(f"Error executing git diff: {e}", file=sys.stderr)
        return results

    current_file = None
    line_num = 0

    for line in proc.stdout.splitlines():
        if line.startswith("+++ b/"):
            current_file = line[6:]
            continue
        if line.startswith("@@"):
            # Format: @@ -old,count +new,count @@
            m = re.search(r"\+(\d+)(?:,(\d+))?", line)
            if m:
                line_num = int(m.group(1))
            continue
        if line.startswith("+") and not line.startswith("+++"):
            added_content = line[1:]
            if current_file and is_text_file(current_file):
                # Skip checking the deslop rules or checker implementation itself
                if "deslop" in current_file.lower():
                    line_num += 1
                    continue
                is_code = current_file.endswith((".go", ".ts", ".js", ".py", ".rs", ".java", ".c", ".cpp"))
                v = check_line(added_content, is_code=is_code)
                for cat, msg in v:
                    results.append((current_file, line_num, added_content.strip(), cat, msg))
            line_num += 1

    return results


def check_commit_diff(commit_ref):
    results = []
    cmd = ["git", "show", "--unified=0", commit_ref]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, check=True)
    except subprocess.CalledProcessError as e:
        print(f"Error executing git show for commit {commit_ref}: {e}", file=sys.stderr)
        return results

    current_file = None
    line_num = 0

    for line in proc.stdout.splitlines():
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
            if current_file and is_text_file(current_file):
                if "deslop" in current_file.lower():
                    line_num += 1
                    continue
                is_code = current_file.endswith((".go", ".ts", ".js", ".py", ".rs", ".java", ".c", ".cpp"))
                v = check_line(added_content, is_code=is_code)
                for cat, msg in v:
                    results.append((current_file, line_num, added_content.strip(), cat, msg))
            line_num += 1

    return results


def get_all_tracked_files():
    cmd = ["git", "ls-files"]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, check=True)
        return [f for f in proc.stdout.splitlines() if is_text_file(f)]
    except subprocess.CalledProcessError:
        return []


def print_report(violations):
    if not violations:
        print("✓ No AI slop detected. Content is clean, direct, and human-to-human.")
        return 0

    print(f"\n❌ Found {len(violations)} instance(s) of AI slop:")
    print("=" * 70)
    for filepath, line_num, content, category, suggestion in violations:
        print(f"File: {filepath}:{line_num}")
        print(f"Category: {category}")
        print(f"Line: \"{content}\"")
        print(f"Guidance: {suggestion}")
        print("-" * 70)

    print("\nTo fix these issues:")
    print("1. Replace em-dashes ('—') with periods, commas, colons, or parentheses.")
    print("2. Replace buzzwords and sugar-coating with direct, plain words.")
    print("3. Maintain technical precision without over-simplification.")
    print("4. Or run the agent skill: /deslop\n")
    return 1


def main():
    parser = argparse.ArgumentParser(description="Scan and deslop text, UI copy, code, and git changes.")
    parser.add_argument("--files", nargs="+", help="Specific files to scan")
    parser.add_argument("--commit", help="Commit SHA or ref to inspect")
    parser.add_argument("--staged", action="store_true", help="Inspect staged changes in git index")
    parser.add_argument("--all", action="store_true", help="Scan all tracked files in the repository")
    parser.add_argument("--force", action="store_true", help="Bypass confirmation when scanning all files")
    parser.add_argument("--rules", help="Path to rules file (e.g. .agents/rules/deslop.md)")

    args = parser.parse_args()

    # Pre-commit hook mode (staged)
    if args.staged:
        violations = check_staged_diff()
        sys.exit(print_report(violations))

    # Commit inspection mode
    if args.commit:
        violations = check_commit_diff(args.commit)
        sys.exit(print_report(violations))

    # Explicit files
    if args.files:
        violations = []
        for f in args.files:
            if os.path.exists(f) and is_text_file(f):
                violations.extend(check_file_content(f))
        sys.exit(print_report(violations))

    # All files
    if args.all:
        if not args.force:
            response = input("No specific file provided. Scan all files in the repository? [y/N]: ").strip().lower()
            if response not in ("y", "yes"):
                print("Aborted.")
                sys.exit(0)
        files = get_all_tracked_files()
        violations = []
        for f in files:
            violations.extend(check_file_content(f))
        sys.exit(print_report(violations))

    # If no arguments provided, ask user
    print("No input specified. Please provide --files, --commit, --staged, or --all.")
    sys.exit(1)


if __name__ == "__main__":
    main()
