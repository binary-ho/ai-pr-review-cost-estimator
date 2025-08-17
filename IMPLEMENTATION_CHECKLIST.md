# PR-Agent Cost Estimator — Divide-and-Conquer Checklist (Concise)

Purpose: A minimal, actionable checklist to guide an AI tool (or engineer) to implement the PRD accurately and efficiently. Keep each step small, testable, and independently verifiable.

## 0) Project Setup
- [x] Define CLI args: `--github-token` (or GITHUB_TOKEN env), `--org`, `--out` (html path), `--since` (optional ISO date), `--until` (optional ISO date).
- [x] Add deps: `google/go-github/v61`, `golang.org/x/oauth2`, `pkoukk/tiktoken-go`.
- [x] Basic project scaffolding: keep `main.go`; created `internal/` packages for api, model (analyze/render/tokenize pending).

## 1) Data Models
- [x] Define core types:
  - [x] RepoSummary: repo name, totalPRs, totalDiffChars, avgDiffCharsPerPR.
  - [x] OrgSummary: repoCount, totalPRs, totalDiffChars, monthsSpan, avgMonthlyPRs, avgMonthlyDiffChars, avgMonthlyTokens, costGPT4oUSD, costClaudeSonnetUSD.
  - [x] TimeRange: firstPRCreatedAt, lastPRCreatedAt, monthsSpan.

## 2) GitHub Client & Utilities
- [x] Authenticated client from token; set User-Agent.
- [x] Pagination helper: basic page iteration for repos.
- [x] Rate limit handling: backoff on 403/RL; respect `X-RateLimit-Remaining` and `Retry-After`.
- [ ] Concurrency controls: worker pool with bounded goroutines; context cancellation support.

## 3) Discover Repositories (P0)
- [x] List all repositories for org (including private): `client.Repositories.ListByOrg` with `Type=all` and pagination.
- [x] Collect minimal fields: name, default branch; store repo list.

## 4) List All PRs per Repository (P0)
- [x] For each repo, list PRs with states: open, closed, merged. Use `PullRequests.List` with `state=all` and pagination.
- [ ] Capture PR number, createdAt, mergedAt/closedAt.
- [x] Track global firstPRDate and lastPRDate across org.

## 5) Fetch Diff Text for Each PR (P0)
- [x] Use GitHub API to get raw diff without cloning:
  - [x] Option A: `client.PullRequests.Get` with `Accept: application/vnd.github.v3.diff` via REST (or `GetRaw` variant if available). (Implemented via GetRaw Diff)
  - Option B: `client.PullRequests.Get` to list files then fetch patch/diff for each file and concatenate.
- [x] Sum diff length in characters per PR; avoid retaining full strings in memory—stream or count length as read.
- [ ] Respect rate limits; add small jitter; parallelize with worker pool.

## 6) Aggregation (P0)
- [x] Per-repo: count PRs; sum diff chars; compute avg per PR.
- [x] Org-wide: repoCount; totalPRs; totalDiffChars.
- [x] Compute monthsSpan from first to last PR inclusive: `months = max(1, (year2-year1)*12 + (month2-month1) + (day2>=day1 ? 0 : 1))` or use precise month diff utility.
- [x] Monthly averages: `avgMonthlyPRs = totalPRs / monthsSpan`; `avgMonthlyDiffChars = totalDiffChars / monthsSpan`.

## 7) Tokenization with tiktoken-go (P0, Accuracy)
- [x] Initialize encoder for GPT-4o equivalent (fallback to cl100k_base).
- [x] Convert `avgMonthlyDiffChars` to an equivalent token count accurately:
  - [x] Strategy A (preferred): sample-based ratio: tokenize representative subset of diffs collected and compute chars→tokens ratio; apply to monthly chars.
  - [ ] Strategy B: if full diffs retained (not recommended), tokenize concatenated monthly sample; otherwise, document estimator method.
- [x] Output `avgMonthlyTokens` as integer.

## 8) Cost Estimation (P0)
- [x] GPT-4o: $5.00 per 1M input tokens → `cost = tokens / 1_000_000 * 5.0`.
- [x] Claude 3.5 Sonnet: $3.00 per 1M input tokens → `cost = tokens / 1_000_000 * 3.0`.
- [x] Round to 2 decimals for display; keep full precision internally.

## 9) HTML Report (P0)
- [x] Use `html/template` to render a single self-contained HTML file.
- [x] Sections: Org Summary, Per-Repository Table.
- [ ] Include basic JS sorting for table columns (P1; progressive enhancement).
- [x] Write to `--out` path; ensure parent dirs exist.

## 10) Performance & Reliability
- [ ] Target: <5 minutes for <100 repos.
- [ ] Tune worker pool size (e.g., 8–32); add timeout per PR diff fetch.
- [ ] Retries with exponential backoff on 5xx/secondary rate limit.
- [ ] Memory: avoid storing all diffs; count lengths on the fly.

Note: Intentionally skipped in Request 7 per instruction (focus on Edge Cases only).

## 11) Edge Cases
- [x] Repos with zero PRs.
- [x] PRs with no diff (empty or reverted).
- [x] Forked/private repos visibility with token scopes (graceful skip per PR on 403/404-class errors).
- [x] Very old first PR date causing huge monthsSpan; clamp by optional `--since/--until` if provided.
- [x] API rate limits and secondary throttling (minimal wait-and-retry to keep runs alive).

## 12) Validation & Success Metrics
- [x] Verify tokenization path exercised; compare sample char→token ratio on known snippets. (See VALIDATION.md)
- [x] Smoketest on small org; ensure report generates. (See VALIDATION.md)
- [x] Check that all P0 items in PRD are present in report. (See VALIDATION.md)

## 13) Runbook
- [x] Build: `go build -o pr-agent-cost-estimator .` (See RUNBOOK.md)
- [x] Run: `GITHUB_TOKEN=... ./pr-agent-cost-estimator --org <ORG> --out report.html [--since 2023-01-01 --until 2025-08-17]` (See RUNBOOK.md)

Notes:
- Accuracy requirement prefers exact tokenization via tiktoken-go. If computing tokens on full monthly text is infeasible, document the sampling approach and expose a flag to increase sample size.
- Keep the checklist minimal—expand only if implementation discovers new constraints.


## Update — Eventual-Complete Mode & Anti-Inf Loop Notes (2025-08-18)
- Rate limit handling enhanced with policy-driven waits and precise 403 classification (only Remaining=0 treated as RL).
- Added flags: --eventual-complete, --max-wait-reset, --sleep-min-ms, --sleep-max-ms, --retries-nonrate.
- Inserted jitter between API calls to reduce secondary RL; PR diff retries now policy-based with exponential backoff.
- Infinite-loop prevention: non-RL 403 (permissions) is skipped; RL waits are capped by policy; context cancellation respected.
