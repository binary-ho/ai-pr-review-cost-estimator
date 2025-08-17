package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	tiktoken "github.com/pkoukk/tiktoken-go"
	api "pr-agent-cost-estimator/internal/api"
	model "pr-agent-cost-estimator/internal/model"
)

type CLIOptions struct {
	GitHubToken      string
	Org              string
	Out              string
	Since            string
	Until            string
	EventualComplete bool
	MaxWaitReset     string
	SleepMinMS       int
	SleepMaxMS       int
	RetriesNonRate   int
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s --org <ORG> --out <REPORT.html> [--github-token <TOKEN>|GITHUB_TOKEN env] [--since YYYY-MM-DD] [--until YYYY-MM-DD]\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	var opts CLIOptions
	flag.StringVar(&opts.GitHubToken, "github-token", "", "GitHub token (or set GITHUB_TOKEN env)")
	flag.StringVar(&opts.Org, "org", "", "GitHub organization to analyze")
	flag.StringVar(&opts.Out, "out", "", "Output HTML report path")
	flag.StringVar(&opts.Since, "since", "", "Optional ISO date (YYYY-MM-DD) to start analysis window")
	flag.StringVar(&opts.Until, "until", "", "Optional ISO date (YYYY-MM-DD) to end analysis window")
	flag.BoolVar(&opts.EventualComplete, "eventual-complete", false, "Wait through rate limit resets and retry pages/PRs until completion")
	flag.StringVar(&opts.MaxWaitReset, "max-wait-reset", "60m", "Maximum wait time for rate-limit reset (e.g., 30m, 60m, 2h). Empty for no limit")
	flag.IntVar(&opts.SleepMinMS, "sleep-min-ms", 200, "Min sleep jitter between API calls (ms)")
	flag.IntVar(&opts.SleepMaxMS, "sleep-max-ms", 800, "Max sleep jitter between API calls (ms)")
	flag.IntVar(&opts.RetriesNonRate, "retries-nonrate", 10, "Retry attempts for non-rate-limit transient errors")
	flag.Usage = usage
	flag.Parse()

	if opts.GitHubToken == "" {
		opts.GitHubToken = os.Getenv("GITHUB_TOKEN")
	}

	// Basic validation for Request 1 (will be tightened in later requests)
	if opts.Org == "" || opts.Out == "" {
		usage()
		os.Exit(2)
	}

	// Parse dates if provided to validate format; ignore errors gracefully for Request 1
	var sincePtr, untilPtr *time.Time
	if opts.Since != "" {
		if t, err := time.Parse("2006-01-02", opts.Since); err == nil {
			sincePtr = &t
		} else {
			fmt.Fprintf(os.Stderr, "Warning: invalid --since format, expected YYYY-MM-DD: %v\n", err)
		}
	}
	if opts.Until != "" {
		if t, err := time.Parse("2006-01-02", opts.Until); err == nil {
			untilPtr = &t
		} else {
			fmt.Fprintf(os.Stderr, "Warning: invalid --until format, expected YYYY-MM-DD: %v\n", err)
		}
	}
	// Configure API policy based on flags
	maxWait := time.Duration(0)
	if opts.MaxWaitReset != "" {
		if d, err := time.ParseDuration(opts.MaxWaitReset); err == nil {
			maxWait = d
		} else {
			fmt.Fprintf(os.Stderr, "Warning: invalid --max-wait-reset, using default 60m: %v\n", err)
			maxWait = 60 * time.Minute
		}
	}
	api.SetPolicy(api.Policy{
		EventualComplete: opts.EventualComplete,
		MaxWaitReset:     maxWait, // 0 means no cap (only if --max-wait-reset "")
		SleepMin:         time.Duration(opts.SleepMinMS) * time.Millisecond,
		SleepMax:         time.Duration(opts.SleepMaxMS) * time.Millisecond,
		RetriesNonRate:   opts.RetriesNonRate,
	})

	// Reference data models to ensure package compiles and is wired
	_ = model.RepoSummary{}
	_ = model.OrgSummary{}
	_ = model.TimeRange{}

	// Request 2: initialize GitHub client and list repositories
	ctx := context.Background()
	client := api.NewGitHubClient(ctx, opts.GitHubToken)
	repos, err := api.ListAllRepos(ctx, client, opts.Org)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing repositories for org %s: %v\n", opts.Org, err)
		os.Exit(1)
	}

	fmt.Printf("Discovered %d repositories in org %s\n", len(repos), opts.Org)
	max := 5
	if len(repos) < max {
		max = len(repos)
	}
	if max > 0 {
		fmt.Println("Sample repos:")
		for i := 0; i < max; i++ {
			name := "<unknown>"
			if repos[i] != nil {
				name = repos[i].GetName()
			}
			fmt.Printf(" - %s\n", name)
		}
	}

	// Request 4: fetch PR diffs per repo, aggregate per-repo and org summaries, and print
	var repoSummaries []model.RepoSummary
	var orgTotalPRs int
	var orgTotalDiffChars int64
	var globalFirst time.Time
	var globalLast time.Time
	// Prepare bounded sample collection for tokenization ratio (200k characters across org)
	var sampleBudget int64 = 200000
	var sampleBuf strings.Builder

	for _, r := range repos {
		if r == nil {
			continue
		}
		repoName := r.GetName()
		prCount, diffChars, first, last, err := api.RepoPRDiffStats(ctx, client, opts.Org, repoName, sincePtr, untilPtr, &sampleBudget, &sampleBuf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to compute diff stats for %s: %v\n", repoName, err)
			continue
		}
		avgPerPR := 0.0
		if prCount > 0 {
			avgPerPR = float64(diffChars) / float64(prCount)
		}
		repoSummaries = append(repoSummaries, model.RepoSummary{
			RepoName:          repoName,
			TotalPRs:          prCount,
			TotalDiffChars:    diffChars,
			AvgDiffCharsPerPR: avgPerPR,
		})
		orgTotalPRs += prCount
		orgTotalDiffChars += diffChars
		if !first.IsZero() && (globalFirst.IsZero() || first.Before(globalFirst)) {
			globalFirst = first
		}
		if !last.IsZero() && (globalLast.IsZero() || last.After(globalLast)) {
			globalLast = last
		}
	}

	computeMonthsSpan := func(first, last time.Time) int {
		if first.IsZero() || last.IsZero() {
			return 0
		}
		y1, m1, d1 := first.Date()
		y2, m2, d2 := last.Date()
		months := (y2-y1)*12 + int(m2-m1)
		if d2 < d1 {
			months++ // include partial month at the end if day hasn't reached
		}
		if months < 1 {
			months = 1
		}
		return months
	}
	monthsSpan := computeMonthsSpan(globalFirst, globalLast)
	avgMonthlyPRs := 0.0
	avgMonthlyDiffChars := 0.0
	if monthsSpan > 0 {
		avgMonthlyPRs = float64(orgTotalPRs) / float64(monthsSpan)
		avgMonthlyDiffChars = float64(orgTotalDiffChars) / float64(monthsSpan)
	}

	// Request 5: Tokenization (tiktoken-go) and Cost Estimation
	var avgMonthlyTokens int64
	var costGPT4oUSD float64
	var costClaudeUSD float64
	if avgMonthlyDiffChars > 0 {
		sampleText := sampleBuf.String()
		if len(sampleText) > 0 {
			enc, err := tiktoken.EncodingForModel("gpt-4o")
			if err != nil || enc == nil {
				// Fallback to cl100k_base if model-specific encoding is not found
				enc, err = tiktoken.GetEncoding("cl100k_base")
			}
			if err == nil && enc != nil {
				toks := enc.Encode(sampleText, nil, nil)
				if len(toks) > 0 {
					ratio := float64(len(toks)) / float64(len(sampleText))
					est := ratio * avgMonthlyDiffChars
					avgMonthlyTokens = int64(math.Round(est))
				}
			}
		}
	}
	// Costs per PRD: GPT-4o $5/M tokens, Claude Sonnet $3/M tokens
	if avgMonthlyTokens > 0 {
		costGPT4oUSD = float64(avgMonthlyTokens) / 1000000.0 * 5.0
		costClaudeUSD = float64(avgMonthlyTokens) / 1000000.0 * 3.0
	}

	windowStr := "all time"
	if sincePtr != nil || untilPtr != nil {
		var sinceStr, untilStr string
		if sincePtr != nil {
			sinceStr = sincePtr.Format("2006-01-02")
		} else {
			sinceStr = "beginning"
		}
		if untilPtr != nil {
			untilStr = untilPtr.Format("2006-01-02")
		} else {
			untilStr = "now"
		}
		windowStr = fmt.Sprintf("%s to %s", sinceStr, untilStr)
	}

	fmt.Printf("\nSummary for %s (window: %s)\n", opts.Org, windowStr)
	fmt.Printf(" - Repositories analyzed: %d\n", len(repos))
	fmt.Printf(" - Total PRs: %d\n", orgTotalPRs)
	fmt.Printf(" - Total diff chars: %d\n", orgTotalDiffChars)
	if orgTotalPRs > 0 {
		fmt.Printf(" - First PR created at: %s\n", globalFirst.Format(time.RFC3339))
		fmt.Printf(" - Last PR created at: %s\n", globalLast.Format(time.RFC3339))
		fmt.Printf(" - Months span (inclusive): %d\n", monthsSpan)
		fmt.Printf(" - Avg monthly PRs: %.2f\n", avgMonthlyPRs)
		fmt.Printf(" - Avg monthly diff chars: %.0f\n", avgMonthlyDiffChars)
		fmt.Printf(" - Avg monthly tokens (est): %d\n", avgMonthlyTokens)
		fmt.Printf(" - Est. monthly cost (GPT-4o): $%.2f\n", costGPT4oUSD)
		fmt.Printf(" - Est. monthly cost (Claude 3.5 Sonnet): $%.2f\n", costClaudeUSD)
	} else {
		fmt.Println(" - No PRs found in the specified window.")
	}

	// Write HTML report
	orgSummary := model.OrgSummary{
		RepoCount:           len(repos),
		TotalPRs:            orgTotalPRs,
		TotalDiffChars:      orgTotalDiffChars,
		MonthsSpan:          monthsSpan,
		AvgMonthlyPRs:       avgMonthlyPRs,
		AvgMonthlyDiffChars: avgMonthlyDiffChars,
		AvgMonthlyTokens:    avgMonthlyTokens,
		CostGPT4oUSD:        costGPT4oUSD,
		CostClaudeSonnetUSD: costClaudeUSD,
	}
	if err := renderHTMLReport(opts, repoSummaries, orgSummary, windowStr); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing HTML report to %s: %v\n", opts.Out, err)
		os.Exit(1)
	}
	fmt.Printf("\nHTML report written to %s\n", opts.Out)

	// Print a small sample of per-repo stats
	if len(repoSummaries) > 0 {
		fmt.Println("\nPer-repo sample:")
		max := 5
		if len(repoSummaries) < max {
			max = len(repoSummaries)
		}
		for i := 0; i < max; i++ {
			rs := repoSummaries[i]
			fmt.Printf(" - %s: PRs=%d, diff chars=%d, avg/PR=%.0f\n", rs.RepoName, rs.TotalPRs, rs.TotalDiffChars, rs.AvgDiffCharsPerPR)
		}
	}
}

// renderHTMLReport writes a single-file HTML report to opts.Out using the computed data.
func renderHTMLReport(opts CLIOptions, repos []model.RepoSummary, org model.OrgSummary, window string) error {
	// Prepare data for template
	type reportData struct {
		OrgName     string
		Window      string
		GeneratedAt string
		Org         model.OrgSummary
		Repos       []model.RepoSummary
	}
	data := reportData{
		OrgName:     opts.Org,
		Window:      window,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Org:         org,
		Repos:       repos,
	}

	// Ensure output directory exists (if any)
	dir := filepath.Dir(opts.Out)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Build template
	const reportHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>{{.OrgName}} — PR Activity & AI Review Cost Report</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, Helvetica, Arial, sans-serif; margin: 2rem; color: #222; }
    h1 { font-size: 1.8rem; margin-bottom: 0.2rem; }
    .sub { color: #555; margin-bottom: 1.2rem; }
    .card { border: 1px solid #eee; border-radius: 8px; padding: 1rem; margin: 1rem 0; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0.8rem; }
    .metric { background: #fafafa; border: 1px solid #eee; border-radius: 8px; padding: 0.8rem; }
    .metric .label { color: #666; font-size: 0.9rem; }
    .metric .value { font-weight: 600; font-size: 1.1rem; }
    table { width: 100%; border-collapse: collapse; font-size: 0.95rem; }
    th, td { text-align: left; padding: 8px; border-bottom: 1px solid #eee; }
    th { background: #f6f6f6; }
    .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace; }
  </style>
</head>
<body>
  <h1>{{.OrgName}} — PR 활동 및 AI 리뷰 비용 예측 리포트</h1>
  <div class="sub">분석 기간: {{.Window}} · 생성 시각: {{.GeneratedAt}}</div>

  <div class="card">
    <h2>📈 조직 전체 요약 (Organization Summary)</h2>
    <div class="grid">
      <div class="metric"><div class="label">총 레포지토리 수</div><div class="value">{{.Org.RepoCount}}</div></div>
      <div class="metric"><div class="label">조직 전체 누적 PR 개수</div><div class="value">{{.Org.TotalPRs}}</div></div>
      <div class="metric"><div class="label">조직 전체 누적 Diff (문자)</div><div class="value mono">{{printf "%d" .Org.TotalDiffChars}}</div></div>
      <div class="metric"><div class="label">개월 수 (첫 PR ~ 마지막 PR)</div><div class="value">{{.Org.MonthsSpan}}</div></div>
      <div class="metric"><div class="label">월 평균 PR 개수</div><div class="value">{{printf "%.2f" .Org.AvgMonthlyPRs}}</div></div>
      <div class="metric"><div class="label">월 평균 Diff (문자)</div><div class="value mono">{{printf "%.0f" .Org.AvgMonthlyDiffChars}}</div></div>
      <div class="metric"><div class="label">월 평균 Diff (토큰 - 정확한 계산)</div><div class="value mono">{{printf "%d" .Org.AvgMonthlyTokens}}</div></div>
      <div class="metric"><div class="label">예상 월 비용 (GPT-4o)</div><div class="value">${{printf "%.2f" .Org.CostGPT4oUSD}}</div></div>
      <div class="metric"><div class="label">예상 월 비용 (Claude 3.5 Sonnet)</div><div class="value">${{printf "%.2f" .Org.CostClaudeSonnetUSD}}</div></div>
    </div>
  </div>

  <div class="card">
    <h2>📂 레포지토리별 상세 통계 (Per-Repository Stats)</h2>
    <table>
      <thead>
        <tr>
          <th>레포지토리</th>
          <th>총 PR 수</th>
          <th>총 Diff (문자)</th>
          <th>PR당 평균 Diff (문자)</th>
        </tr>
      </thead>
      <tbody>
        {{range .Repos}}
        <tr>
          <td class="mono">{{.RepoName}}</td>
          <td>{{.TotalPRs}}</td>
          <td class="mono">{{printf "%d" .TotalDiffChars}}</td>
          <td class="mono">{{printf "%.0f" .AvgDiffCharsPerPR}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
  </div>

  <div class="sub">본 리포트는 GitHub API와 tiktoken-go 기반 추정치를 사용하여 생성되었습니다.</div>
</body>
</html>`

	tmpl, err := template.New("report").Parse(reportHTML)
	if err != nil {
		return err
	}

	f, err := os.Create(opts.Out)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, data)
}
