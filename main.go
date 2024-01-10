package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

func main() {
	owner := "codeready-toolchain"
	repo := "member-operator"

	// Create a new GitHub client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	numPRs := 5 * 2 // Number of PRs to fetch (It will fetch twice, just because you might don't have enough merged PRs). Set to 0 to fetch all PRs.
	opt := getPullRequestListOptions(numPRs)

	// Fetch the closed pull requests
	var allPRs []*github.PullRequest
	for {
		prs, resp, err := client.PullRequests.List(ctx, owner, repo, opt)
		if err != nil {
			fmt.Println("Error fetching pull requests:", err)
			return
		}
		allPRs = append(allPRs, prs...)
		if (numPRs > 0 && len(allPRs) >= numPRs) || resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	// Trim the slice to the desired number of PRs
	if numPRs > 0 && len(allPRs) > numPRs {
		allPRs = allPRs[:numPRs]
	}

	// Print the merge times for each PR
	prInfos := getMergeTimes(ctx, client, owner, repo, allPRs)
	printPRInfos(prInfos)

	// Print the number and title of each closed pull request
	// for _, pr := range allPRs {
	// 	fmt.Printf("#%d: %s\n", *pr.Number, *pr.Title)
	// }
}

func getPullRequestListOptions(numPRs int) *github.PullRequestListOptions {
	perPage := 100 // Fetch 100 PRs per page
	if numPRs > 0 && numPRs < 100 {
		perPage = numPRs
	}
	return &github.PullRequestListOptions{
		State: "closed",
		ListOptions: github.ListOptions{
			PerPage: perPage,
		},
	}
}

type PRInfo struct {
	Number                      int
	Title                       string
	Creator                     string
	CreationDayOfWeek           string
	CreationTimeOfDay           string
	FirstResponder              string
	FirstResponseDayOfWeek      string
	FirstResponseTimeOfDay      string
	TimeToFirstResponse         time.Duration
	FirstHumanResponder         string
	FirstHumanResponseDayOfWeek string
	FirstHumanResponseTimeOfDay string
	TimeToFirstHumanResponse    time.Duration
	MergeDayOfWeek              string
	MergeTimeOfDay              string
	Quarter                     string
	Year                        int
	Duration                    time.Duration
	Commits                     int
	Commenters                  []string
	Reviewers                   []string
}

func getMergeTimes(ctx context.Context, client *github.Client, owner string, repo string, prs []*github.PullRequest) []PRInfo {
	prInfos := make([]PRInfo, 0)

	for _, pr := range prs {
		if pr.MergedAt != nil && pr.CreatedAt != nil {
			var prInfo PRInfo
			prInfo.Number = *pr.Number
			prInfo.Title = *pr.Title
			prInfo.Creator = *pr.User.Login
			prInfo.CreationDayOfWeek, prInfo.CreationTimeOfDay = getDayOfWeekAndTimeOfDay(pr.CreatedAt.UTC())
			prInfo.Duration = pr.MergedAt.Sub(*pr.CreatedAt)
			prInfo.Year, prInfo.Quarter = getYearAndQuarter(*pr.CreatedAt)

			// Fetch the comments for the PR
			comments, _, err := client.Issues.ListComments(ctx, owner, repo, *pr.Number, nil)
			if err != nil {
				fmt.Printf("Error fetching comments for PR #%d: %s\n", *pr.Number, err)
				continue
			}

			// Calculate the time to first response and first human response
			for _, comment := range comments {
				if prInfo.FirstResponder == "" {
					prInfo.TimeToFirstResponse = comment.CreatedAt.Sub(*pr.CreatedAt)
					prInfo.FirstResponseDayOfWeek, prInfo.FirstResponseTimeOfDay = getDayOfWeekAndTimeOfDay(comment.CreatedAt.UTC())
					prInfo.FirstResponder = *comment.User.Login
				}
				if !strings.HasSuffix(*comment.User.Login, "[bot]") && prInfo.FirstHumanResponder == "" {
					prInfo.TimeToFirstHumanResponse = comment.CreatedAt.Sub(*pr.CreatedAt)
					prInfo.FirstHumanResponseDayOfWeek, prInfo.FirstHumanResponseTimeOfDay = getDayOfWeekAndTimeOfDay(comment.CreatedAt.UTC())
					prInfo.FirstHumanResponder = *comment.User.Login
					break
				}
			}

			// Fetch the commits for the PR
			commits, _, err := client.PullRequests.ListCommits(ctx, owner, repo, *pr.Number, nil)
			if err != nil {
				fmt.Printf("Error fetching commits for PR #%d: %s\n", *pr.Number, err)
				continue
			}
			prInfo.Commits = len(commits)

			// Get the names of the developers who created the PR, reviewed it, and wrote comments
			for _, comment := range comments {
				if !strings.HasSuffix(*comment.User.Login, "[bot]") {
					prInfo.Commenters = append(prInfo.Commenters, *comment.User.Login)
				}
			}

			// Fetch the reviews for the PR
			reviews, _, err := client.PullRequests.ListReviews(ctx, owner, repo, *pr.Number, &github.ListOptions{})
			if err != nil {
				fmt.Printf("Error fetching reviews for PR #%d: %s\n", *pr.Number, err)
				continue
			}

			// Get the names of the reviewers
			for _, review := range reviews {
				if !strings.HasSuffix(*review.User.Login, "[bot]") {
					prInfo.Reviewers = append(prInfo.Reviewers, *review.User.Login)
				}
			}

			prInfo.MergeDayOfWeek, prInfo.MergeTimeOfDay = getDayOfWeekAndTimeOfDay(pr.MergedAt.UTC())

			prInfos = append(prInfos, prInfo)
		}
	}

	return prInfos
}

func getDayOfWeekAndTimeOfDay(t time.Time) (dayOfWeek string, timeOfDay string) {
	dayOfWeek = t.Weekday().String()
	switch hour := t.Hour(); {
	case hour < 6:
		timeOfDay = "after midnight"
	case hour < 12:
		timeOfDay = "morning"
	case hour < 17:
		timeOfDay = "afternoon"
	case hour < 20:
		timeOfDay = "evening"
	default:
		timeOfDay = "night"
	}
	return
}

func printPRInfos(prInfos []PRInfo) {
	for _, prInfo := range prInfos {
		firstHumanResponseMessage := "did not have a first human response"
		if prInfo.FirstHumanResponder != "" {
			firstHumanResponseMessage = fmt.Sprintf("had a first human response by %s on a %s in the %s after %v", prInfo.FirstHumanResponder, prInfo.FirstHumanResponseDayOfWeek, prInfo.FirstHumanResponseTimeOfDay, prInfo.TimeToFirstHumanResponse)
		}

		fmt.Printf("PR #%d: %s was created by %s on a %s in the %s, had a first response by %s on a %s in the %s after %v, %s, was merged on a %s in the %s in %s-%d, took %v to merge, included %d commits, and had %d review comments by %v, reviewed by %d people %v\n",
			prInfo.Number, prInfo.Title, prInfo.Creator, prInfo.CreationDayOfWeek, prInfo.CreationTimeOfDay, prInfo.FirstResponder, prInfo.FirstResponseDayOfWeek, prInfo.FirstResponseTimeOfDay, prInfo.TimeToFirstResponse, firstHumanResponseMessage, prInfo.MergeDayOfWeek, prInfo.MergeTimeOfDay, prInfo.Quarter, prInfo.Year, prInfo.Duration, prInfo.Commits, len(prInfo.Commenters), prInfo.Commenters, len(prInfo.Reviewers), prInfo.Reviewers)
	}
}

func getYearAndQuarter(t time.Time) (int, string) {
	year := t.Year()
	quarter := "Q1"
	switch {
	case t.Month() >= 4 && t.Month() <= 6:
		quarter = "Q2"
	case t.Month() >= 7 && t.Month() <= 9:
		quarter = "Q3"
	case t.Month() >= 10:
		quarter = "Q4"
	}
	return year, quarter
}
