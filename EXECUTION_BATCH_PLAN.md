# Implementation Request Batching Plan (for Junie AI)

Purpose: Map how many checklist items (0..13 in IMPLEMENTATION_CHECKLIST.md) to complete per request, balancing dependencies, API/network latency, and Junie’s reliability.

Guiding principles:
- Group by dependency chain and complexity; avoid mixing heavy network features with large refactors in the same request.
- Keep each request shippable and verifiable (builds, runs, or provides visible output).
- Prioritize P0 end-to-end path early, then refine performance/edge cases.

## Request 1 — Items 0, 1
- 0) Project Setup
- 1) Data Models
Rationale: Establish CLI, deps, scaffolding, and core types. Minimal risk, unblocks all later steps.
DoD:
- go build succeeds; flags parsed; types compile; basic project layout in place.

## Request 2 — Items 2, 3
- 2) GitHub Client & Utilities
- 3) Discover Repositories (P0)
Rationale: Implement authenticated client, pagination helpers, worker pool, then list all repos. Natural dependency chain, moderate effort.
DoD:
- Auth works with token; lists org repos (Type=all) with pagination; dry-run prints repo count.

## Request 3 — Item 4
- 4) List All PRs per Repository (P0)
Rationale: PR enumeration is sizeable (pagination, date filtering, concurrency). Keep focus to ensure correctness and rate-limit handling.
DoD:
- For discovered repos, enumerate PRs state=all; capture dates; support optional --since/--until; print total PRs and global first/last PR dates.

## Request 4 — Items 5, 6
- 5) Fetch Diff Text for Each PR (P0)
- 6) Aggregation (P0)
Rationale: These are tightly coupled. Fetching diffs and computing char counts feeds aggregation; shipping together yields the first meaningful analytics output.
DoD:
- For each PR, fetch diff via API without cloning; count diff chars; compute per-repo/org totals and averages; log summary to stdout.

## Request 5 — Items 7, 8
- 7) Tokenization with tiktoken-go (P0, Accuracy)
- 8) Cost Estimation (P0)
Rationale: Once monthly char metrics exist, implement accurate token conversion and costs. Computationally light, logically cohesive.
DoD:
- Tokenization path implemented (encoder initialized); char→token estimator (sample-based or direct) applied; monthly token and cost figures computed.

## Request 6 — Item 9
- 9) HTML Report (P0)
Rationale: Render a single-file HTML report after core metrics are available. Keep isolated to focus on template and formatting.
DoD:
- Generates self-contained HTML at --out; includes Org Summary and per-repo table.

## Request 7 — Item 11 (Edge Cases only; skip 10)
- 11) Edge Cases
Rationale: Per instruction, skip performance (item 10); focus only on correctness-oriented edge cases (zero-PR, empty diffs, visibility, minimal throttling handling).
DoD:
- Graceful behavior on 403/404/410/451 per-PR diff errors (skip diff, keep counting PRs); minimal wait-and-retry for rate limits; report is generated even with partial skips.

## Request 8 — Items 12, 13
- 12) Validation & Success Metrics
- 13) Runbook
Rationale: Final validation and documentation to make the tool usable and verifiable for others.
DoD:
- Smoketest results; tokenization sanity checks; runbook commands finalized in docs.

Notes:
- P1 sorting for the report can be scheduled after Request 6 as an optional follow-up.
- If rate limits are tight in your org, split Request 4 into 4A (fetch diffs) and 4B (aggregation) as separate submissions.
