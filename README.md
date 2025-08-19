# PR-Agent Cost Estimator — 사용 방법 (Quick Start)

이 도구는 GitHub Organization의 모든 저장소(PR 포함)를 분석하여, PR의 diff 텍스트 기준 월 평균 토큰(정확한 tiktoken 계산)과 예상 월별 비용(GPT-4o, Claude 3.5 Sonnet)을 추정해 단일 HTML 리포트를 생성합니다.

## 1) 준비물 (Prerequisites)
- Go 1.21+ (모듈 타깃: 1.25) — `go version`으로 확인
- GitHub Personal Access Token (classic) — `repo` 스코프 필요 (Private repo 포함 분석하려면 필수)
- 네트워크로 api.github.com 접근 가능

## 2) 빌드 (Build)
```bash
go build -o pr-agent-cost-estimator .
```
빌드가 완료되면 프로젝트 루트에 `./pr-agent-cost-estimator` 바이너리가 생성됩니다.

## 3) 실행 (Run)
필수: Organization 이름(`--org`)과 출력 HTML 경로(`--out`). 토큰은 플래그 또는 환경변수 `GITHUB_TOKEN`로 전달합니다.

```bash
# 예시 1) 전체 기간 분석 (소규모 Org 권장)
GITHUB_TOKEN=xxxx \
./pr-agent-cost-estimator \
  --org <ORG_NAME> \
  --out report.html

# 예시 2) 기간 제한 분석 (권장)
GITHUB_TOKEN=xxxx \
./pr-agent-cost-estimator \
  --org <ORG_NAME> \
  --out out/report-2024H2.html \
  --since 2024-07-01 --until 2024-12-31

# 예시 3) 최신 모델로 비용 비교 (무제한 지정)
GITHUB_TOKEN=xxxx \
./pr-agent-cost-estimator \
  --org <ORG_NAME> \
  --out out/report-latest.html \
  --encoding-model gpt-4.1 \
  --pricing "GPT-5:5.50" \
  --pricing "Sonnet 4:3.20" \
  --pricing "Opus:2.00"
```

### 지원 플래그
- `--org` (필수): 분석할 GitHub Organization 로그인
- `--out` (필수): 생성할 HTML 리포트 파일 경로
- `--github-token` (선택): 토큰을 플래그로 직접 전달 (미지정 시 `GITHUB_TOKEN` 사용)
- `--since` / `--until` (선택): 분석 기간(YYYY-MM-DD). 미지정 시 전체 이력 분석
- 모델/가격 옵션:
  - `--encoding-model` (기본 gpt-4o): tiktoken 인코딩 모델명. 예) gpt-4.1, gpt-4o, gpt-5, o200k_base, cl100k_base
  - `--pricing "이름:USD_per_M"` (반복 지정): 표시할 모델과 100만 토큰당 USD 단가를 임의 개수만큼 지정. 예) `--pricing "GPT-5:5.5" --pricing "Sonnet 4:3.2" --pricing "Opus:2.0"`
  - 플래그 미지정 시 기본으로 GPT-4o($5/M), Claude 3.5 Sonnet($3/M)이 표시됩니다.
- 고급(완결 모드 관련):
  - `--eventual-complete` (기본 false): 레이트리밋에 걸리면 리셋 시간까지 기다렸다가 같은 요청을 반복하여 “끝까지” 완료를 지향합니다.
  - `--max-wait-reset` (기본 60m): 레이트리밋 대기 상한(예: 30m, 60m, 2h). 빈 문자열이면 상한 없음.
  - `--sleep-min-ms` / `--sleep-max-ms` (기본 200/800): API 호출 간 지터 범위(ms). secondary rate limit 완화용.
  - `--retries-nonrate` (기본 10): 레이트리밋이 아닌 일시 오류(5xx/네트워크)에 대한 재시도 횟수.

## 4) 출력 (What you get)
- 표준출력(stdout):
  - 저장소 수, 총 PR 수, 총 diff 문자 수
  - 첫/마지막 PR 시각, 개월 수, 월 평균 PR 수/문자 수
  - 월 평균 토큰(정확한 tiktoken 기반 샘플 비율 적용) 및 예상 월 비용(GPT-4o, Claude)
- HTML 리포트(`--out` 경로):
  - 조직 요약(Repo 수, 총/월 평균 지표, 토큰 및 비용 추정치)
  - 저장소별 상세 통계(총 PR 수, 총 Diff, 평균 Diff/PR)

## 5) 동작 및 예외 처리
- 접근 권한 부족 등으로 특정 PR의 diff를 가져올 수 없는 경우(403/404/410/451) 해당 PR의 diff만 건너뛰고 나머지를 계속 처리합니다.
- API Rate Limit에 도달하면 `Retry-After` 또는 Rate Reset 시간까지 잠시 대기 후 재시도합니다.
- 모든 diff 전문을 메모리에 보관하지 않고 길이만 합산하며, tiktoken 토큰화 비율 계산을 위해 조직 단위로 최대 약 200k자 샘플만 보관합니다.

## 6) 문제 해결 (Troubleshooting)
- "Error listing repositories": 토큰 `repo` 스코프 및 Org 이름 확인
- 403/404가 많이 발생: 토큰의 접근 권한이 부족할 수 있음 (Org 멤버십/Private 접근 권한 확인)
- 실행이 느림: 현재 성능 튜닝은 범위 외. `--since`/`--until`로 기간을 좁혀보세요
- 빈 리포트/0 값들: 기간이 활동을 모두 제외했을 수 있음 — 기간을 넓혀 실행

## 7) 더 자세한 문서
- RUNBOOK: 빌드/실행/출력/트러블슈팅 상세 — ./RUNBOOK.md
- VALIDATION: 스모크 테스트와 정확도 검증 가이드 — ./VALIDATION.md

---
질문이 있거나 문제가 발생하면 이슈로 남겨주세요. 빠르게 도와드리겠습니다!