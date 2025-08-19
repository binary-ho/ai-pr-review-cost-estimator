# Model/Pricing Refactor — Divide & Conquer Plan

Goal: Allow unlimited cost models to be selected via CLI flags, and choose tokenization encoding via CLI at runtime. Support latest names like GPT‑5, Sonnet 4, Opus without code changes.

Scope Outcomes
- Add flags: --encoding-model, --pricing "Name:USD_per_M" (repeatable)
- Tokenization encoding switched by flag; fallback to o200k_base → cl100k_base
- Generalized pricing/parsing and dynamic rendering in stdout/HTML
- No limit on number of models; defaults preserved for backward compatibility

Phases
1) CLI & Types
- Add fields to CLIOptions: EncModel string; Pricing []string
- Register flags: --encoding-model (default gpt-4o), --pricing (repeatable)
- Introduce types: Price{Name, USDPerM}, CostRow{Name, MonthlyUSD}

2) Tokenization
- Use tiktoken.EncodingForModel(EncModel)
- If not found, try GetEncoding("o200k_base") → GetEncoding("cl100k_base")
- Keep existing sample-based chars→tokens ratio approach

3) Pricing & Costs
- Parse each --pricing entry as Name:USD_per_M (float)
- If none provided, default to [GPT-4o:$5/M, Claude 3.5 Sonnet:$3/M]
- Compute []CostRow from avgMonthlyTokens
- stdout: print costs as a list

4) HTML Render
- Add Costs []CostRow to report data
- Replace fixed two labels with range .Costs
- Keep OrgSummary’s cost fields as zeros (compat); display is from Costs

5) Documentation
- README & RUNBOOK update: flags description and examples
- Usage example for GPT‑5 / Sonnet 4 / Opus

6) Validation
- Build success
- No flags: defaults shown (2 rows)
- Multiple --pricing entries: all rendered in stdout/HTML
- Wrong --pricing format: warning, ignored
- Different --encoding-model values change token counts scale plausibly

Notes & Risks
- tiktoken-go encodings are OpenAI-centric; absolute token counts may differ for other vendors; we document this
- Extremely many pricing rows can widen the summary grid; acceptable for now (P1: responsive layout)
