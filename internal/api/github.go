package api

import (
	"context"
	github "github.com/google/go-github/v61/github"
	"golang.org/x/oauth2"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const userAgent = "pr-agent-cost-estimator/0.1"

// Policy controls API call behavior (wait/retry/jitter) and is configurable from main.
type Policy struct {
	EventualComplete bool          // If true, wait through rate limit resets until completion
	MaxWaitReset     time.Duration // Cap per wait for rate reset; 0 means no cap
	SleepMin         time.Duration // Min jitter between API calls
	SleepMax         time.Duration // Max jitter between API calls
	RetriesNonRate   int           // Retries for transient non-rate-limit errors
}

var policy = Policy{
	EventualComplete: false,
	MaxWaitReset:     2 * time.Minute,
	SleepMin:         0,
	SleepMax:         0,
	RetriesNonRate:   1,
}

// SetPolicy sets the global API policy.
func SetPolicy(p Policy) { policy = p }

func sleepJitter() {
	if policy.SleepMax <= 0 {
		return
	}
	min := policy.SleepMin
	max := policy.SleepMax
	if max < min {
		max = min
	}
	delta := max - min
	var extra time.Duration
	if delta > 0 {
		extra = time.Duration(rand.Int63n(int64(delta)))
	}
	time.Sleep(min + extra)
}

// NewGitHubClient creates an authenticated GitHub client if token is provided; otherwise unauthenticated.
func NewGitHubClient(ctx context.Context, token string) *github.Client {
	var httpClient *http.Client
	if token != "" {
		// Use OAuth2 transport with static token
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		httpClient = oauth2.NewClient(ctx, ts)
	}
	client := github.NewClient(httpClient)
	client.UserAgent = userAgent
	return client
}

// ListAllRepos lists all repositories for the given org with Type=all, handling pagination.
func ListAllRepos(ctx context.Context, client *github.Client, org string) ([]*github.Repository, error) {
	opt := &github.RepositoryListByOrgOptions{Type: "all", ListOptions: github.ListOptions{PerPage: 100}}
	var all []*github.Repository
	for {
		repos, resp, err := client.Repositories.ListByOrg(ctx, org, opt)
		if err != nil {
			if resp != nil && waitIfRateLimited(resp) {
				// retry same page after waiting
				continue
			}
			return nil, err
		}
		all = append(all, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
		sleepJitter()
	}
	return all, nil
}

// CountPRsAndDateRange enumerates all PRs for a repo (state=all) with pagination and optional since/until
// filtering on PR creation time. It returns the count of PRs within the window and the earliest and latest
// createdAt timestamps observed (zero values if none).
func CountPRsAndDateRange(ctx context.Context, client *github.Client, owner, repo string, since, until *time.Time) (int, time.Time, time.Time, error) {
	opt := &github.PullRequestListOptions{
		State:       "all",
		Sort:        "created",
		Direction:   "asc",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	count := 0
	var first time.Time
	var last time.Time
	for {
		prs, resp, err := client.PullRequests.List(ctx, owner, repo, opt)
		if err != nil {
			if resp != nil && waitIfRateLimited(resp) {
				// retry same page after waiting
				continue
			}
			return 0, time.Time{}, time.Time{}, err
		}
		for _, pr := range prs {
			createdTS := pr.GetCreatedAt()
			created := createdTS.Time
			if !created.IsZero() {
				if since != nil && created.Before(*since) {
					continue
				}
				if until != nil && created.After(*until) {
					continue
				}
				count++
				if first.IsZero() || created.Before(first) {
					first = created
				}
				if last.IsZero() || created.After(last) {
					last = created
				}
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
		sleepJitter()
	}
	return count, first, last, nil
}

// RepoPRDiffStats lists PRs (state=all) for a repo within an optional createdAt window and
// fetches the raw diff for each PR to compute the total diff character count.
// It returns (prCount, totalDiffChars, firstCreated, lastCreated).
func RepoPRDiffStats(ctx context.Context, client *github.Client, owner, repo string, since, until *time.Time, sampleBudget *int64, sampleBuf *strings.Builder) (int, int64, time.Time, time.Time, error) {
	opt := &github.PullRequestListOptions{
		State:       "all",
		Sort:        "created",
		Direction:   "asc",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	count := 0
	var total int64
	var first time.Time
	var last time.Time
	for {
		prs, resp, err := client.PullRequests.List(ctx, owner, repo, opt)
		if err != nil {
			if resp != nil && waitIfRateLimited(resp) {
				// retry same page after waiting
				continue
			}
			return 0, 0, time.Time{}, time.Time{}, err
		}
		for _, pr := range prs {
			created := pr.GetCreatedAt().Time
			if created.IsZero() {
				continue
			}
			if since != nil && created.Before(*since) {
				continue
			}
			if until != nil && created.After(*until) {
				continue
			}
			count++
			if first.IsZero() || created.Before(first) {
				first = created
			}
			if last.IsZero() || created.After(last) {
				last = created
			}

			// Fetch raw diff for the PR and sum its length in characters (graceful on errors)
			var diff string
			var rresp *github.Response
			var derr error
			// policy-based retries for non-rate-limit errors
			attempts := policy.RetriesNonRate
			if attempts < 1 {
				attempts = 1
			}
			backoff := 1 * time.Second
			for {
				diff, rresp, derr = client.PullRequests.GetRaw(ctx, owner, repo, pr.GetNumber(), github.RawOptions{Type: github.Diff})
				if derr == nil {
					break
				}
				if rresp != nil && waitIfRateLimited(rresp) {
					// rate limit: wait according to policy and retry (no attempt decrement)
					continue
				}
				if isSkippableClientError(rresp) {
					// permission/visibility/etc.: skip this PR diff
					derr = nil
					diff = ""
					break
				}
				attempts--
				if attempts <= 0 || ctx.Err() != nil {
					// give up on this PR, skip
					diff = ""
					derr = nil
					break
				}
				time.Sleep(backoff)
				if backoff < 2*time.Minute {
					backoff *= 2
				}
			}
			if derr != nil {
				// still failing after retries, skip PR
				continue
			}
			l := int64(len(diff))
			total += l

			// Optionally collect a bounded sample of diff text for tokenization ratio calculation
			if sampleBudget != nil && sampleBuf != nil {
				if *sampleBudget > 0 {
					toTake := l
					if toTake > *sampleBudget {
						toTake = *sampleBudget
					}
					if toTake > 0 {
						// Convert to int for slicing; safe since budget is kept reasonably small by caller
						end := int(toTake)
						if end > len(diff) {
							end = len(diff)
						}
						sampleBuf.WriteString(diff[:end])
						*sampleBudget -= int64(end)
					}
				}
			}
			sleepJitter()
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
		sleepJitter()
	}
	return count, total, first, last, nil
}

// waitIfRateLimited sleeps for the duration indicated by Retry-After header or Rate.Reset.
// Returns true if it waited and the caller should retry; false otherwise.
func waitIfRateLimited(resp *github.Response) bool {
	if resp == nil || resp.Response == nil {
		return false
	}
	if !isRateLimitResponse(resp) {
		return false
	}
	// Prefer Retry-After seconds if present
	if v := resp.Response.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			waitWithCap(time.Duration(secs) * time.Second)
			return true
		}
	}
	// Fallback to Rate.Reset time
	if !resp.Rate.Reset.Time.IsZero() {
		wait := time.Until(resp.Rate.Reset.Time)
		if wait <= 0 {
			wait = 5 * time.Second
		}
		waitWithCap(wait)
		return true
	}
	return false
}

// isRateLimitResponse determines whether the response indicates hitting rate limits.
func isRateLimitResponse(resp *github.Response) bool {
	if resp == nil || resp.Response == nil {
		return false
	}
	code := resp.Response.StatusCode
	if code == 429 {
		return true
	}
	if code == 403 {
		// Only treat as rate limit if remaining is 0
		return resp.Response.Header.Get("X-RateLimit-Remaining") == "0"
	}
	return false
}

func waitWithCap(wait time.Duration) {
	capDur := policy.MaxWaitReset
	if !policy.EventualComplete {
		// For non-eventual mode, default to 2m cap if none provided
		if capDur == 0 {
			capDur = 2 * time.Minute
		}
	}
	if capDur > 0 && wait > capDur {
		wait = capDur
	}
	time.Sleep(wait)
}

// isSkippableClientError returns true for client-side errors we want to skip per-PR.
func isSkippableClientError(resp *github.Response) bool {
	if resp == nil || resp.Response == nil {
		return false
	}
	code := resp.Response.StatusCode
	switch code {
	case 403:
		// Skip only if it's not a rate limit 403
		return !isRateLimitResponse(resp)
	case 404, 410, 451:
		return true
	default:
		return false
	}
}
