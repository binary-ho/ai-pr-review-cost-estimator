# PR-Agent Cost Estimator — Validation & Success Metrics

This document provides a lightweight validation guide to ensure the tool works as intended and meets PRD P0 accuracy and reporting requirements.

## 1) Smoketest (Small Org or Narrow Window)
Run the tool against a small organization or a narrow date window to minimize API usage:

```
# Windowed run (example dates)
GITHUB_TOKEN=xxxx \
./pr-agent-cost-estimator \
  --org <ORG> \
  --out out/report-smoketest.html \
  --since 2024-07-01 --until 2024-08-31
```

Validate the following:
- The program prints a summary to stdout with:
  - Repositories analyzed (>= 1)
  - Total PRs (>= 0)
  - Total diff chars (>= 0)
  - First/last PR timestamps (when PRs exist)
  - Months span (>= 1 when PRs exist)
  - Avg monthly PRs and diff chars (>= 0)
  - Avg monthly tokens (>= 0 when monthly diff > 0)
  - Estimated monthly costs (>= 0 when tokens > 0)
- The HTML file at `--out` exists and opens in a browser.
  - Organization Summary metrics are populated.
  - Per-repository table lists repos with totals and averages.

Edge-case tolerance (expected):
- Some PR diffs may be skipped with warnings (403/404/410/451). Report should still render.
- If the window has no PRs, report shows 0 metrics and a “No PRs found” message in stdout.

## 2) Tokenization Sanity Checks
The tool computes a chars→tokens ratio using a bounded sample of diffs (~200k chars across the org) with tiktoken-go. Validate that:
- When Avg Monthly Diff Chars > 0, the "Avg monthly tokens" in stdout/HTML is > 0.
- Implied ratio = AvgMonthlyTokens / AvgMonthlyDiffChars is within a plausible range (typically 0.2–0.6 for code diffs; depends on content and encoding).
- Re-run with a larger activity window (or a more active org) and observe that tokens scale roughly proportionally with monthly diff chars.

Optional deeper check (advanced):
- Run two analyses with different windows where monthly diff chars differ significantly. Verify that the token estimate scales with chars.
- If variance is high, it likely reflects different code/text composition. This is acceptable; the mechanism is sampling-based.

## 3) PRD P0 Feature Presence Checklist
Confirm the HTML report and stdout reflect the following PRD P0 items:
- Data Collection:
  - All org repositories are discovered (private included with proper token).
  - All PRs (state=all) are considered in the selected window.
  - Each PR’s diff is fetched via API (no cloning); failures are skipped per-PR without aborting the run.
- Analysis:
  - Per-repo: total PRs, total diff chars, average diff chars per PR.
  - Org-wide: repo count, total PRs, total diff chars.
  - Timespan: months from first PR to last PR in the window; monthly averages for PRs and diff chars.
- Tokenization & Cost:
  - Monthly avg diff chars converted to tokens via tiktoken-go (GPT-4o encoding or cl100k_base fallback).
  - Monthly cost estimates shown for GPT-4o ($5/M) and Claude 3.5 Sonnet ($3/M).
- Reporting:
  - Single-file HTML report with Org Summary and per-repo table.

## 4) Success Metrics
- Accuracy: Tokenization uses tiktoken-go; estimates are derived from sampled diffs rather than a fixed chars→tokens heuristic.
- Usability: A single run produces a browsable HTML report with the core org stats required for decision-making.
- Stability: Encountering some inaccessible PRs or temporary rate limits does not abort the run; report still generates.

## 5) Troubleshooting Pointers
- 403/404 on many diffs: ensure the token has access to the repos and PRs (repo scope, and org membership if needed).
- Rate limits: the tool waits on Retry-After or rate reset and continues; narrow the date window if necessary.
- Empty or near-zero metrics: the time window may exclude most PRs; widen the window.

For run instructions and more details, see RUNBOOK.md.
