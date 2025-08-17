# PR-Agent Cost Estimator — Runbook

This runbook explains how to build and run the tool, what inputs are required, and how to interpret results.

## Prerequisites
- Go 1.21+ (module target is 1.25). Verify with: `go version`.
- A GitHub Personal Access Token (classic) with `repo` scope to include private repositories. Set it in env as `GITHUB_TOKEN`.
- Network access to api.github.com.

## Build
```
go build -o pr-agent-cost-estimator .
```
This produces a single binary `pr-agent-cost-estimator` in the project root.

## Run
Basic usage:
```
GITHUB_TOKEN=xxxxxxxx \
./pr-agent-cost-estimator \
  --org <ORG_NAME> \
  --out report.html \
  [--since 2023-01-01] [--until 2025-08-17]
```
Flags:
- `--org` (required): GitHub organization login to analyze.
- `--out` (required): Path to write the HTML report.
- `--github-token` (optional): Token via flag; if omitted, the tool reads `GITHUB_TOKEN` from the environment.
- `--since` / `--until` (optional): Analysis window (YYYY-MM-DD). If omitted, analyzes all available history.
- Advanced (eventual-complete mode):
  - `--eventual-complete` (default false): When hitting rate limits, wait until reset and retry the same request to eventually complete, rather than skipping.
  - `--max-wait-reset` (default 60m): Cap on a single wait for rate reset (e.g., 30m, 60m, 2h). Empty string means no cap.
  - `--sleep-min-ms` / `--sleep-max-ms` (default 200/800): Jitter (ms) inserted between API calls to avoid secondary rate limits.
  - `--retries-nonrate` (default 10): Retry attempts for transient non-rate-limit errors (5xx/network), with exponential backoff.

### Output
- The tool prints a summary to stdout (repo count, total PRs, total diff chars, months span, monthly averages, estimated monthly tokens and costs).
- It writes a single-file HTML report at the `--out` path with:
  - Organization Summary metrics.
  - Per-repository totals and averages.

### Behavior and Edge Cases
- Repositories with zero PRs are handled gracefully (reported as 0s).
- PR diffs that cannot be fetched due to permissions or other client errors (403/404/410/451) are skipped per-PR; the run continues.
- If GitHub rate limits are hit, the tool will wait briefly (honoring `Retry-After` or rate reset) and retry.
- Diffs are not stored in full; lengths are counted and a small bounded sample (≈200k chars across org) is retained to compute a chars→tokens ratio using tiktoken-go.

### Cost Estimation
- Tokenization uses `tiktoken-go` (GPT-4o encoding or `cl100k_base` fallback) to compute a representative chars→tokens ratio from sampled diffs, which is then applied to average monthly diff characters.
- Costs:
  - GPT-4o: $5.00 per 1,000,000 input tokens.
  - Claude 3.5 Sonnet: $3.00 per 1,000,000 input tokens.

## Troubleshooting
- `Error listing repositories` ⇒ Ensure the token has `repo` scope and the org name is correct.
- Many skipped diffs with 403/404 ⇒ The token lacks access to some private repos or PRs; the report will still be generated with available data.
- Slow runs ⇒ Performance tuning is out-of-scope per current instructions; consider narrowing the window with `--since`/`--until`.
- Empty report or zero PRs ⇒ The org might be inactive or the time window filters out all data.

## Examples
All-time analysis (recommended only for smaller orgs):
```
GITHUB_TOKEN=xxxx ./pr-agent-cost-estimator --org my-company --out report.html
```
Windowed analysis:
```
GITHUB_TOKEN=xxxx ./pr-agent-cost-estimator --org my-company --out out/report-2024H2.html --since 2024-07-01 --until 2024-12-31
```

## Notes
- GitHub Enterprise Server is not supported (only github.com).
- The HTML report is static and self-contained. Sorting is P1 and not yet implemented.
