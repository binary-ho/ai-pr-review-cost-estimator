package model

import "time"

// RepoSummary holds per-repository aggregated metrics.
type RepoSummary struct {
	RepoName          string  `json:"repoName"`
	TotalPRs          int     `json:"totalPRs"`
	TotalDiffChars    int64   `json:"totalDiffChars"`
	AvgDiffCharsPerPR float64 `json:"avgDiffCharsPerPR"`
}

// OrgSummary holds organization-wide aggregated metrics and cost estimates.
type OrgSummary struct {
	RepoCount           int     `json:"repoCount"`
	TotalPRs            int     `json:"totalPRs"`
	TotalDiffChars      int64   `json:"totalDiffChars"`
	MonthsSpan          int     `json:"monthsSpan"`
	AvgMonthlyPRs       float64 `json:"avgMonthlyPRs"`
	AvgMonthlyDiffChars float64 `json:"avgMonthlyDiffChars"`
	AvgMonthlyTokens    int64   `json:"avgMonthlyTokens"`
	CostGPT4oUSD        float64 `json:"costGPT4oUSD"`
	CostClaudeSonnetUSD float64 `json:"costClaudeSonnetUSD"`
}

// TimeRange tracks the first and last PR dates and the computed month span.
type TimeRange struct {
	FirstPRCreatedAt time.Time `json:"firstPRCreatedAt"`
	LastPRCreatedAt  time.Time `json:"lastPRCreatedAt"`
	MonthsSpan       int       `json:"monthsSpan"`
}
