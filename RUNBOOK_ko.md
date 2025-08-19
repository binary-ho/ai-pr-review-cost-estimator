# PR-Agent Cost Estimator — Runbook

이 문서는 도구를 빌드하고 실행하는 방법, 필요한 입력값, 그리고 결과를 해석하는 방법을 설명합니다.

## Prerequisites
- Go 1.21+ (module target 1.25). `go version`으로 확인하세요.
- GitHub Personal Access Token (classic) — private repository를 포함하려면 `repo` scope가 필요합니다. 환경변수 `GITHUB_TOKEN`에 설정하세요.
- api.github.com에 대한 네트워크 접근 권한.

## Build
```
go build -o pr-agent-cost-estimator .
```
프로젝트 루트에 단일 바이너리 `pr-agent-cost-estimator`가 생성됩니다.

## Run
기본 사용 예:
```
GITHUB_TOKEN=xxxxxxxx \
./pr-agent-cost-estimator \
  --org <ORG_NAME> \
  --out report.html \
  [--since 2023-01-01] [--until 2025-08-17]
```
Flags:
- `--org` (required): 분석할 GitHub organization 로그인.
- `--out` (required): HTML 리포트를 기록할 경로.
- `--github-token` (optional): 플래그로 토큰 전달. 생략 시 환경변수 `GITHUB_TOKEN`을 읽습니다.
- `--since` / `--until` (optional): 분석 기간(YYYY-MM-DD). 생략 시 사용 가능한 전체 이력을 분석합니다.
- Model/Pricing options:
  - `--encoding-model` (default gpt-4o): tiktoken encoding 모델명. 예: gpt-4.1, gpt-4o, gpt-5, o200k_base, cl100k_base.
  - `--pricing "Name:USD_per_M"` (repeatable): 1M input tokens당 비용을 원하는 만큼 추가. 예: `--pricing "GPT-5:5.5" --pricing "Sonnet 4:3.2" --pricing "Opus:2.0"`.
  - 미지정 시 기본값: GPT-4o ($5/M), Claude 3.5 Sonnet ($3/M).
- Advanced (eventual-complete mode):
  - `--eventual-complete` (default false): rate limit에 걸리면 skip 대신 reset까지 대기 후 동일 요청을 재시도하여 결국 완료를 지향.
  - `--max-wait-reset` (default 60m): 단일 rate reset 대기 상한(예: 30m, 60m, 2h). 빈 문자열이면 상한 없음.
  - `--sleep-min-ms` / `--sleep-max-ms` (default 200/800): API 호출 간 삽입할 지터(ms). secondary rate limit을 피하기 위함.
  - `--retries-nonrate` (default 10): non-rate-limit(5xx/네트워크) 일시 오류에 대한 재시도 횟수(지수 백오프).

### Output
- 표준출력(stdout)에 요약을 출력합니다(레포 수, 총 PR 수, 총 diff 문자 수, 개월 수, 월간 평균, 추정 월간 tokens 및 비용).
- `--out` 경로에 단일 HTML 리포트를 생성합니다:
  - Organization Summary 지표.
  - 레포지토리별 합계 및 평균.

### Behavior and Edge Cases
- PR가 0개인 repository도 정상 처리됩니다(0으로 보고).
- 권한 또는 기타 클라이언트 오류(403/404/410/451)로 가져올 수 없는 PR diff는 PR 단위로 건너뛰고 실행을 계속합니다.
- GitHub rate limit에 도달하면 `Retry-After` 또는 rate reset을 존중하여 잠시 대기 후 재시도합니다.
- 전체 diff 본문은 저장하지 않으며, 길이만 합산합니다. 또한 조직 단위로 제한된 작은 샘플(약 200k chars)을 보관하여 tiktoken-go로 chars→tokens 비율을 계산합니다.

### Cost Estimation
- Tokenization은 `tiktoken-go`(GPT-4o encoding 또는 `cl100k_base` 폴백)를 사용하여 수집된 샘플 diff로 대표적인 chars→tokens 비율을 구하고, 이를 월 평균 diff 문자 수에 적용합니다.
- 비용:
  - GPT-4o: 1,000,000 input tokens당 $5.00
  - Claude 3.5 Sonnet: 1,000,000 input tokens당 $3.00

## Troubleshooting
- `Error listing repositories` ⇒ 토큰에 `repo` scope가 있는지, org 이름이 정확한지 확인하세요.
- 403/404로 많은 diff가 스킵됨 ⇒ 일부 private repo 또는 PR에 대한 접근 권한이 부족할 수 있습니다. 사용 가능한 데이터로 리포트는 계속 생성됩니다.
- 실행이 느림 ⇒ 현재 지침상 성능 튜닝은 범위 밖입니다. `--since`/`--until`로 기간을 좁혀보세요.
- 리포트가 비어 있거나 PR이 0 ⇒ org가 비활성 상태이거나 지정한 기간이 모든 데이터를 걸러냈을 수 있습니다.

## Examples
All-time 분석(소규모 org에만 권장):
```
GITHUB_TOKEN=xxxx ./pr-agent-cost-estimator --org my-company --out report.html
```
기간 제한 분석:
```
GITHUB_TOKEN=xxxx ./pr-agent-cost-estimator --org my-company --out out/report-2024H2.html --since 2024-07-01 --until 2024-12-31
```
Latest models cost comparison (unlimited entries):
```
GITHUB_TOKEN=xxxx \
./pr-agent-cost-estimator \
  --org my-company \
  --out out/report-latest.html \
  --encoding-model gpt-4.1 \
  --pricing "GPT-5:5.50" \
  --pricing "Sonnet 4:3.20" \
  --pricing "Opus:2.00"
```

## Notes
- GitHub Enterprise Server는 지원하지 않습니다(github.com만 지원).
- HTML 리포트는 정적이며 self-contained입니다. 정렬(sorting)은 P1로 아직 구현되어 있지 않습니다.
