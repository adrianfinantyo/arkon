---
name: deslop
description: "Detect and eliminate AI slop—such as em-dashes, corporate buzzwords, sycophancy, and boilerplate comments—from text, UI copy, code, or logic. Rewrites content to sound natural, grounded, and human-to-human without over-simplifying technical depth. Use when asked to deslop, humanize, clean up writing, or eliminate AI tropes."
---

# Deslop Skill

Detect and eliminate AI-generated writing habits, filler, and decorative fluff from text, UI copy, code, and logic. Transform content into clear, direct, human-to-human communication while keeping technical rigor and precision intact.

## Input Resolution

Determine the target before making any modifications:

1. **Explicit files mentioned:** Target those specific files directly.
2. **Commit or ref specified:** (e.g., `HEAD`, `HEAD~1`, branch name): Inspect the diff using `git diff <commit>...HEAD` or `git show <commit>` and target changed files.
3. **Staged changes:** (e.g., pre-commit check): Inspect staged changes via `git diff --cached`.
4. **No target specified:** **Do NOT scan the entire project automatically.** Stop and ask the user for confirmation first:
   > "No specific files or commit were mentioned. Would you like me to scan all tracked files in the repository?"
   Proceed with full-repo deslop only after explicit user confirmation.

## The Slop Taxonomy

Audit the target content across five dimensions:

### 1. Dashes & Rhetorical Punctuation
- **Zero em-dashes (`—`) or en-dashes (` – `) in prose:** Models lean heavily on em-dashes as a rhetorical crutch to insert breathless parentheticals or dramatic conclusions.
- **Replacement:** Use a period to break the sentence into two clear thoughts. Use commas for natural clauses. Use parentheses for incidental technical details. Use a colon to introduce an explanation.
- **Avoid double-hyphen dividers (`--`) in text:** Use standard punctuation instead.

### 2. Buzzwords & AI Markers
Eliminate inflated corporate vocabulary and AI cliches. Replace with plain, direct words:
- `delve` / `delving` → explore, inspect, look into, examine
- `leverage` / `leveraging` → use, apply, rely on
- `is a testament to` → shows, proves, reflects
- `tapestry` / `rich tapestry` → combination, variety, system, mix
- `seamless` / `seamlessly` → smooth, direct, automatic
- `beacon` → model, example, guide
- `game-changer` → major improvement, significant shift
- `meticulous` / `meticulously` → thorough, careful, detailed
- `crucial` / `paramount` / `pivotal` → key, vital, important, required
- `multifaceted` / `holistic` → comprehensive, broad, varied
- `foster` / `empower` → enable, help, allow, support
- `revolutionize` / `cutting-edge` → modernize, update, improve
- `deep dive` / `dive deep` → detailed look, thorough analysis
- `plethora` / `myriad` → many, several, numerous
- `nestled` / `unravel` → located, clarify, resolve

### 3. Sugar-Coating & Sycophancy
- **No throat-clearing:** Cut openings like "Certainly!", "I'd be delighted to help", "Great question!", or "In today's fast-paced world...". State the point immediately.
- **No performative cheerleading:** Remove unearned hype ("revolutionary", "breathtaking", "exciting news!"). State what the feature or system does factually.
- **No formulaic conclusion summaries:** Cut robotic wraps ("In conclusion, ...", "Overall, by doing X, we ensure Y...", "It is important to remember..."). When the explanation is complete, stop.
- **Human-to-human voice:** Write like two pragmatic software engineers discussing a design problem over coffee. Candid, grounded, respectful, and clear.

### 4. Code, UI & Logic Slop
- **Tautological comments:** Delete comments that merely restate the syntax (e.g., `// increment i`, `// check if user is nil`, `// return the response`).
- **Defensive bloat:** Remove paranoia checks on internal private invariants that earlier code or type systems already guarantee.
- **Boilerplate indirection:** Strip single-use wrapper classes or unnecessary adapter layers that only forward arguments without adding logic.
- **UI copy:** Error messages must tell the user what went wrong, why, and how to fix it—not "Oops! Something went wrong!". Action buttons should use specific verbs ("Save photo", "Copy link").

### 5. Depth Invariant: Do NOT Over-Simplify
- **Clarity is not dumbing down:** Human-to-human communication between technical peers does not mean "explain like I'm 5".
- **Preserve domain and technical precision:** Retain exact hardware specs (e.g., CPU models, cache sizes, memory footprints), architectural patterns, database isolation levels, algorithm complexity ($O(n)$ bounds), and domain-specific terms.
- **Keep trade-offs honest:** Real engineering involves compromises. Don't smooth over rough edges or limitations with cheerful hand-waving.

## Execution Workflow

1. **Resolve input:** Identify targets (mentioned files, commit ref, or ask confirmation for `--all`).
2. **Run deslop checker:**
   ```bash
   ./scripts/deslop.py --files <targets>
   # or for staged files:
   ./scripts/deslop.py --staged
   ```
3. **Review & Rewrite:** Apply edits directly to the offending sections following the taxonomy.
4. **Verify:**
   - Run the checker again to confirm zero violations.
   - Verify that technical substance, domain models, and system nuance are 100% intact.
