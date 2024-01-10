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
		timeOfDay = "after midnight [UTC 00:00-06:00)"
	case hour < 12:
		timeOfDay = "morning [UTC 06:00-12:00)"
	case hour < 17:
		timeOfDay = "afternoon [UTC 12:00-17:00)"
	case hour < 20:
		timeOfDay = "evening [UTC 17:00-20:00)"
	default:
		timeOfDay = "night [UTC 20:00-00:00)"
	}
	return
}

func printPRInfos(prInfos []PRInfo) {

	// print the average merge time
	fmt.Printf("Average merge time: %v\n", averageMergeTime(prInfos))

	// print the average time to first human response
	fmt.Printf("Average time to first human response: %v\n", averageFirstReponseHumanTime(prInfos))

	// print the average time to first bot response
	fmt.Printf("Average time to first bot response: %v\n", averageTimeToFirstBotResponse(prInfos))

	// print the average number of comments
	fmt.Printf("Average number of comments per PR: %v\n", averageNumberOfComments(prInfos))

	// print the average number of reviewers
	fmt.Printf("Average number of reviewers per PR: %v\n", averageNumberOfReviewers(prInfos))

	// print the average number of commits
	fmt.Printf("Average number of commits per PR: %v\n", averageNumberOfCommits(prInfos))

	// print the day of the week with the most PRs created
	fmt.Printf("Day of the week with the most PRs created: %s\n", dayWithMostPRsCreated(prInfos))

	// print the time of the day with the most PRs created
	fmt.Printf("Time of the day with the most PRs created: %s\n", timeOfTheDayWithMostPRsCreated(prInfos))

	// print the day of the week with the most PRs merged
	fmt.Printf("Day of the week with the most PRs merged: %s\n", dayMostPRsMerged(prInfos))

	// print the time of the day with the most PRs merged
	fmt.Printf("Time of the day with the most PRs merged: %s\n", timeOfTheDayWithMostPRsMerged(prInfos))

	// print the day of the week with the most first human responses
	fmt.Printf("Day of the week with the most first human responses: %s\n", dayOfTheWeekWithMostFirstHumanResponses(prInfos))

	// print the time of the day with the most first human responses
	fmt.Printf("Time of the day with the most first human responses: %s\n", timeOfTheDayWithMostFirstHumanResponses(prInfos))

	// print the day of the week with the most PR reviews
	fmt.Printf("Day of the week with the most PR reviews: %s\n", dayOfTheWeekWithMostPRReviews(prInfos))

	// print the time of the day with the most PR reviews
	fmt.Printf("Time of the day with the most PR reviews: %s\n", timeOfTheDayWithMostPRReviews(prInfos))

	// print the names of all developers who created, merged, reviewed, commented on, or approved PRs
	fmt.Printf("Names of all developers who created, merged, reviewed, commented on, or approved PRs: %v\n", getTheNamesOfAllDevelopersWhoCreatedMergedReviewedCommentedOnOrApprovedPRs(prInfos))

	// print the top reviewer
	fmt.Printf("Top reviewer: %s\n", getTopReviewer(prInfos))

	// print the top commenter
	fmt.Printf("Top commenter: %s\n", getTopCommenter(prInfos))

	// print the top creator
	fmt.Printf("Top creator: %s\n", getTopCreator(prInfos))

	// print the top first human responder
	fmt.Printf("Top first human responder: %s\n", getTopFirstHumanResponder(prInfos))

	// print the top first responder
	fmt.Printf("Top first responder: %s\n", getTopFirstResponder(prInfos))

	// print the top merger
	fmt.Printf("Top merger: %s\n", getTopMerger(prInfos))

	fmt.Println("----------------------------------------")

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

func averageMergeTime(prData []PRInfo) time.Duration {
	if len(prData) == 0 {
		return time.Duration(0)
	}

	var total time.Duration
	for _, pr := range prData {
		total += pr.Duration
	}

	return total / time.Duration(len(prData))
}

func averageFirstReponseHumanTime(prData []PRInfo) time.Duration {
	if len(prData) == 0 {
		return time.Duration(0)
	}

	var total time.Duration
	for _, pr := range prData {
		total += pr.TimeToFirstHumanResponse
	}

	return total / time.Duration(len(prData))
}

func averageNumberOfComments(prData []PRInfo) float64 {
	if len(prData) == 0 {
		return 0
	}

	var total int
	for _, pr := range prData {
		total += len(pr.Commenters)
	}

	return float64(total) / float64(len(prData))
}

func averageTimeToFirstBotResponse(prData []PRInfo) time.Duration {
	if len(prData) == 0 {
		return time.Duration(0)
	}

	var total time.Duration
	for _, pr := range prData {
		total += pr.TimeToFirstResponse
	}

	return total / time.Duration(len(prData))
}

func averageNumberOfReviewers(prData []PRInfo) float64 {
	if len(prData) == 0 {
		return 0
	}

	var total int
	for _, pr := range prData {
		total += len(pr.Reviewers)
	}

	return float64(total) / float64(len(prData))
}

func averageNumberOfCommits(prData []PRInfo) float64 {
	if len(prData) == 0 {
		return 0
	}

	var total int
	for _, pr := range prData {
		total += pr.Commits
	}

	return float64(total) / float64(len(prData))
}

func dayWithMostPRsCreated(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	days := make(map[string]int)
	for _, pr := range prData {
		days[pr.CreationDayOfWeek]++
	}

	max := 0
	maxDay := ""
	for day, count := range days {
		if count > max {
			max = count
			maxDay = day
		}
	}

	return maxDay
}

func timeOfTheDayWithMostPRsCreated(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	times := make(map[string]int)
	for _, pr := range prData {
		times[pr.CreationTimeOfDay]++
	}

	max := 0
	maxTime := ""
	for time, count := range times {
		if count > max {
			max = count
			maxTime = time
		}
	}

	return maxTime
}

func dayMostPRsMerged(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	days := make(map[string]int)
	for _, pr := range prData {
		days[pr.MergeDayOfWeek]++
	}

	max := 0
	maxDay := ""
	for day, count := range days {
		if count > max {
			max = count
			maxDay = day
		}
	}

	return maxDay
}

func timeOfTheDayWithMostPRsMerged(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	times := make(map[string]int)
	for _, pr := range prData {
		times[pr.MergeTimeOfDay]++
	}

	max := 0
	maxTime := ""
	for time, count := range times {
		if count > max {
			max = count
			maxTime = time
		}
	}

	return maxTime
}

func dayOfTheWeekWithMostFirstHumanResponses(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	days := make(map[string]int)
	for _, pr := range prData {
		if pr.FirstHumanResponder != "" {
			days[pr.FirstHumanResponseDayOfWeek]++
		}
	}

	max := 0
	maxDay := ""
	for day, count := range days {
		if count > max {
			max = count
			maxDay = day
		}
	}

	return maxDay
}

func timeOfTheDayWithMostFirstHumanResponses(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	times := make(map[string]int)
	for _, pr := range prData {
		if pr.FirstHumanResponder != "" {
			times[pr.FirstHumanResponseTimeOfDay]++
		}
	}

	max := 0
	maxTime := ""
	for time, count := range times {
		if count > max {
			max = count
			maxTime = time
		}
	}

	return maxTime
}

func getTheNamesOfAllDevelopersWhoCreatedMergedReviewedCommentedOnOrApprovedPRs(prData []PRInfo) []string {
	if len(prData) == 0 {
		return []string{}
	}

	developers := make(map[string]bool)
	for _, pr := range prData {
		developers[pr.Creator] = true
		for _, commenter := range pr.Commenters {
			developers[commenter] = true
		}
		for _, reviewer := range pr.Reviewers {
			developers[reviewer] = true
		}
	}

	var names []string
	for name := range developers {
		names = append(names, name)
	}

	return names
}

func dayOfTheWeekWithMostPRReviews(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	days := make(map[string]int)
	for _, pr := range prData {
		days[pr.FirstResponseDayOfWeek]++
	}

	max := 0
	maxDay := ""
	for day, count := range days {
		if count > max {
			max = count
			maxDay = day
		}
	}

	return maxDay
}

func timeOfTheDayWithMostPRReviews(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	times := make(map[string]int)
	for _, pr := range prData {
		times[pr.FirstResponseTimeOfDay]++
	}

	max := 0
	maxTime := ""
	for time, count := range times {
		if count > max {
			max = count
			maxTime = time
		}
	}

	return maxTime
}

// Create a struct for each developer to hold the data of interest
type Developer struct {
	Name                                    string
	PRsCreated                              int
	PRsMerged                               int
	PRsReviewed                             int
	PRsCommentedOn                          int
	PRsFirstHumanResponse                   int
	DayWithMostPRsCreated                   time.Weekday
	TimeOfTheDayWithMostPRsCreated          string
	DayWithMostPRsMerged                    time.Weekday
	TimeOfTheDayWithMostPRsMerged           string
	DayOfTheWeekWithMostFirstHumanResponses time.Weekday
	TimeOfTheDayWithMostFirstHumanResponses string
	DayOfTheWeekWithMostPRReviews           time.Weekday
	TimeOfTheDayWithMostPRReviews           string
	DayOfTheWeekWithMostPRComments          time.Weekday
	TimeOfTheDayWithMostPRComments          string
}

func getTopReviewer(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	reviewers := make(map[string]int)
	for _, pr := range prData {
		for _, reviewer := range pr.Reviewers {
			reviewers[reviewer]++
		}
	}

	max := 0
	maxReviewer := ""
	for reviewer, count := range reviewers {
		if count > max {
			max = count
			maxReviewer = reviewer
		}
	}

	return maxReviewer
}

func getTopCommenter(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	commenters := make(map[string]int)
	for _, pr := range prData {
		for _, commenter := range pr.Commenters {
			commenters[commenter]++
		}
	}

	max := 0
	maxCommenter := ""
	for commenter, count := range commenters {
		if count > max {
			max = count
			maxCommenter = commenter
		}
	}

	return maxCommenter
}

func getTopCreator(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	creators := make(map[string]int)
	for _, pr := range prData {
		creators[pr.Creator]++
	}

	max := 0
	maxCreator := ""
	for creator, count := range creators {
		if count > max {
			max = count
			maxCreator = creator
		}
	}

	return maxCreator
}

func getTopFirstHumanResponder(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	firstHumanResponders := make(map[string]int)
	for _, pr := range prData {
		if pr.FirstHumanResponder != "" {
			firstHumanResponders[pr.FirstHumanResponder]++
		}
	}

	max := 0
	maxFirstHumanResponder := ""
	for firstHumanResponder, count := range firstHumanResponders {
		if count > max {
			max = count
			maxFirstHumanResponder = firstHumanResponder
		}
	}

	return maxFirstHumanResponder
}

func getTopFirstResponder(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	firstResponders := make(map[string]int)
	for _, pr := range prData {
		firstResponders[pr.FirstResponder]++
	}

	max := 0
	maxFirstResponder := ""
	for firstResponder, count := range firstResponders {
		if count > max {
			max = count
			maxFirstResponder = firstResponder
		}
	}

	return maxFirstResponder
}

func getTopMerger(prData []PRInfo) string {
	if len(prData) == 0 {
		return ""
	}

	mergers := make(map[string]int)
	for _, pr := range prData {
		mergers[pr.Creator]++
	}

	max := 0
	maxMerger := ""
	for merger, count := range mergers {
		if count > max {
			max = count
			maxMerger = merger
		}
	}

	return maxMerger
}
