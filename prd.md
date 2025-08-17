제품 요구사항 명세서(PRD): GitHub PR 분석 및 비용 예측 도구
문서 버전: 1.0
작성일: 2025년 8월 17일

# 1. 개요 및 문제 정의 (Overview & Problem Statement)
## 1.1. 배경
   개발팀의 코드 리뷰 프로세스 효율화를 위해 AI 기반 PR 리뷰 자동화 도구(PR Agent) 도입을 검토하고 있습니다. 오픈소스 PR Agent를 직접 설치(self-hosting)하여 사용할 경우, 비용의 대부분은 코드 변경사항(diff)을 LLM API로 전송할 때 발생하는 토큰 사용료입니다.

## 1.2. 문제 정의
"우리 조직은 한 달에 평균적으로 어느 정도의 코드 변경(diff)을 만들어내며, 이를 기반으로 AI 리뷰 도구를 사용할 경우 월별 예상 비용은 얼마인가?"
이 질문에 대한 데이터 기반의 답변 없이는 '도구 도입'과 '자체 개발' 사이의 합리적인 의사결정이 어렵습니다. 현재 조직의 전체 PR 통계 및 코드 변경량을 한눈에 파악할 수 있는 방법이 부재합니다.

# 2. 목표 및 기대 효과 (Goals & Objectives)
   2.1. 제품 목표
   핵심 목표: GitHub Organization 내 모든 레포지토리의 Pull Request 데이터를 분석하여, AI 리뷰 Agent 사용 시 월별 예상 비용을 산출한다.

부가 목표: 레포지토리별 개발 활동(PR 수, 코드 변경량)을 정량적으로 파악하고 비교할 수 있는 시각적 리포트를 제공한다.

## 2.2. 기대 효과
AI 리뷰 도구 도입에 대한 데이터 기반의 명확한 'Build vs. Buy' 의사결정 지원

향후 LLM API 예산 책정을 위한 기초 자료 확보

조직 내 각 프로젝트의 활동성을 객관적인 데이터로 파악

# 3. 사용자 (Target Audience)
   개발팀 리더 / 엔지니어링 매니저: 팀의 생산성 도구 도입을 결정하고 예산을 관리하는 역할

DevOps / 플랫폼 엔지니어: AI 리뷰 도구를 직접 설치하고 운영할 담당자

# 4. 기능 요구사항 (Features & Requirements)
## 4.1. 데이터 수집 (Data Collection)
   [P0] Org 내 모든 레포지토리 조회: GitHub API를 통해 분석 대상 Organization에 속한 모든 레포지토리(Private 포함) 목록을 자동으로 가져와야 한다.

[P0] 전체 PR 데이터 조회: 각 레포지토리의 생성부터 현재까지 모든 PR(상태: open, closed, merged) 목록을 가져와야 한다.

[P0] PR별 코드 변경(diff) 수집: 각 PR의 diff 내용을 API를 통해 텍스트 형태로 수집해야 한다. (로컬에 clone 불필요)

## 4.2. 데이터 분석 및 집계 (Data Analysis)
[P0] 레포지토리별 통계 집계:

총 PR 개수

총 diff 문자 수

PR 1개당 평균 diff 문자 수

[P0] 조직 전체 통계 집계:

분석 대상 총 레포지토리 수

조직 전체 누적 PR 개수

조직 전체 누적 diff 문자 수

조직의 첫 PR 생성일부터 마지막 PR 생성일까지의 개월 수 계산

월 평균 PR 개수

월 평균 diff 문자 수 (가장 중요한 지표)

## 4.3. 비용 예측 (Cost Estimation)
[P0] 토큰 변환 (개선됨): 집계된 '월 평균 diff 문자열'을 tiktoken-go 라이브러리를 사용하여 OpenAI 모델(GPT-4o) 기준의 정확한 토큰(Token) 수로 변환해야 한다.

~~변환 기준: 4 characters ≈ 1 token (코드 및 영문 기준)~~ (더 이상 사용 안 함)

[P0] 월 예상 비용 계산: 변환된 월 평균 토큰 수를 기준으로 아래 모델들의 API 비용을 각각 계산하여 제시해야 한다.

OpenAI GPT-4o (1M 입력 토큰당 $5.00)

Anthropic Claude 3.5 Sonnet (1M 입력 토큰당 $3.00)

참고: Claude 모델의 토큰 계산 방식은 OpenAI와 다르지만, 비용 예측의 일관성을 위해 GPT-4o 기준 토큰 수를 공통으로 사용한다.

## 4.4. 리포트 생성 (Reporting)
[P0] HTML 리포트 생성: 모든 분석 결과를 담은 단일 HTML 파일을 생성해야 한다.

[P1] 정렬 기능: 레포지토리별 통계 테이블은 'PR 개수', '총 diff' 등의 컬럼을 기준으로 정렬 가능해야 한다.
# 5. 기술 명세 (Technical Specifications)
   프로그래밍 언어: Go (Golang)

선정 이유: 단일 실행 파일로 컴파일되어 배포가 간편하고, 동시성(goroutine)을 활용해 다수의 레포지토리와 PR을 병렬로 빠르게 처리하는 데 유리함.

핵심 라이브러리:

GitHub API 연동: google/go-github

정확한 토큰 계산: pkoukk/tiktoken-go

HTML 템플릿: html/template (Go 표준 라이브러리)

실행 환경: CLI (Command Line Interface)

실행 시 GitHub Token, Org 이름, 출력 파일명 등을 인자(argument)로 받는다.

# 6. 리포트 UI/UX (Mockup)
[조직 이름] PR 활동 및 AI 리뷰 비용 예측 리포트
분석 기간: 2023-01-01 ~ 2025-08-17

📈 조직 전체 요약 (Organization Summary)
지표 (Metric)

값 (Value)

총 레포지토리 수

85개

월 평균 PR 개수

250개

월 평균 Diff (문자)

7,500,000자

월 평균 Diff (토큰 - 정확한 계산)

1,952,000 토큰

예상 월 비용 (GPT-4o)

$9.76

예상 월 비용 (Claude 3.5 Sonnet)

$5.86

📂 레포지토리별 상세 통계 (Per-Repository Stats)
레포지토리 명 (Repository) ▼

총 PR 수 (Total PRs) ▼

총 Diff (문자 수) ▼

PR당 평균 Diff (문자) ▼

project-alpha

1,204

25,480,120

21,162.89

project-beta

850

18,230,450

21,447.59

common-library

312

5,600,800

17,951.28

...

...

...

...

# 7. 성공 지표 (Success Metrics)
   정확성 (강화됨): 스크립트가 반환하는 예상 비용이 tiktoken 라이브러리 기준에 따라 정확하게 계산되는가?

실행 속도: 100개 미만의 레포지토리를 가진 조직 분석 시 5분 이내에 리포트 생성이 완료되는가?

의사결정 지원: 생성된 리포트가 AI 리뷰 도구 도입 논의에 핵심 근거 자료로 활용되는가?

# 8. 가정 및 제약사항 (Assumptions & Constraints)
   ~~가정: 코드 diff의 문자 수가 LLM API 비용과 정비례 관계를 가진다고 가정한다.~~ (가정 불필요)

제약사항: 이 도구는 GitHub.com에서만 작동하며, GitHub Enterprise Server는 지원하지 않는다.

의존성: 실행을 위해서는 repo 스코프 권한을 가진 GitHub Personal Access Token이 반드시 필요하다.