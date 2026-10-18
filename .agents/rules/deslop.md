---
trigger: always_on
description: "Rules to avoid AI slop: eliminates long dashes (em-dashes), buzzwords, and excessive sugar-coating across documentation, UI copy, and code while preserving technical depth."
---

# Anti-Slop (Deslop) Rules

Code, documentation, UI copy, and commit messages must read like they were written by a thoughtful human communicating directly with another human. Avoid AI-generated writing habits, filler, and decorative complexity.

## 1. Punctuation & Dashes

- **No em-dashes (`—`) or en-dashes used as em-dashes (` – `)** in prose, docs, UI, or commit messages. AI models use em-dashes obsessively as a dramatic rhetorical crutch.
- **Natural human alternatives:** Use a period to split into two crisp sentences, use commas for natural pauses, use parentheses for incidental remarks, or use a colon to introduce an explanation.
- **Hyphens (`-`):** Permitted only for compound adjectives (e.g., `real-world`, `self-hosted`), list bullets, or CLI flags. Avoid double-hyphens (`--`) as sentence dividers in prose.

## 2. Ban AI Buzzwords & Stilted Markers

Eliminate AI cliches and inflated corporate vocabulary. Replace them with plain, direct words:

| Banned / AI-Typical | Preferred Human Alternative |
| :--- | :--- |
| `delve`, `delving into` | explore, inspect, examine, look into |
| `leverage`, `leveraging` | use, apply, rely on |
| `is a testament to` | shows, proves, reflects |
| `tapestry`, `rich tapestry` | combination, system, variety, mix |
| `seamless`, `seamlessly` | smooth, direct, automatic, without friction |
| `beacon`, `stands as a beacon` | example, model, guide |
| `game-changer` | major improvement, significant shift |
| `meticulous`, `meticulously` | careful, thorough, detailed |
| `crucial`, `paramount`, `pivotal` | key, vital, important, needed |
| `multifaceted`, `holistic` | comprehensive, broad, varied |
| `foster`, `empower` | enable, allow, help, support |
| `revolutionize`, `cutting-edge` | modernize, improve, updated |
| `dive deep`, `deep dive` | detailed look, thorough analysis |
| `plethora`, `myriad` | many, several, numerous |
| `nestled`, `unravel` | located, clarify, resolve |

## 3. Eliminate Sugar-Coating & Sycophancy

- **No conversational throat-clearing:** Never start explanations with "Certainly!", "I'd be glad to help", "Great question!", or "In today's fast-paced world...". Get straight to the point.
- **No performative enthusiasm:** Avoid unearned superlatives ("revolutionary", "magical", "breathtaking", "exciting update!"). State what happened or how the system works without cheerleading.
- **No canned concluding summaries:** Cut formulaic summary endings like "In conclusion, ...", "Overall, by doing X, we ensure Y...", or "It is important to remember that...". If the points have been stated, stop writing.
- **Human-to-human tone:** Write frankly, grounded, and respectfully. Frame explanations the way two pragmatic engineers discuss an architecture problem over coffee.

## 4. UI Copy Standards

- **Error messages:** State what failed, why it failed (if known), and what the user can do. Avoid robotic tropes like "Oops! Something went wrong. An unexpected error occurred."
- **Action labels:** Use concrete action verbs ("Save photo", "Copy link", "Download archive") rather than vague marketing labels ("Get started now!", "Experience the magic").
- **Toasts and notifications:** Short, informative, neutral. "Upload complete" instead of "Hooray! Your files have been successfully uploaded to the cloud!"

## 5. Code & Logic Simplicity

- **No tautological comments:** Delete comments that restate the code (e.g., `// increment counter`, `// user id`, `// check if user is nil`). Comments should explain *why* an unusual decision was made, not *what* the syntax says.
- **No boilerplate indirection:** Do not add single-use helper functions, redundant wrapper classes, or speculative interfaces that only forward arguments.
- **No defensive paranoia:** Avoid validating impossible internal invariants when the type system or earlier assertions already guarantee the state.

## 6. Preserve Technical Nuance (Do NOT Over-Simplify)

- **Clarity is not dumbing down:** Eliminating fluff does not mean removing technical substance.
- **Preserve exact domain terms:** Hardware models (e.g., `Intel Pentium Dual-Core E5400`), throughput metrics (e.g., `64KB streaming chunks`), architecture patterns (e.g., `expand-and-contract migrations`), and database invariants must remain precise.
- **Preserve trade-offs and edge cases:** When explaining a design decision, keep the trade-offs, constraints, and limitations intact. Real systems have trade-offs; AI slop glosses over them with cheerful platitudes.
