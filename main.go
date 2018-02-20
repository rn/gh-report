package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Period defines a time period
type Period struct {
	Start time.Time
	End   time.Time
}

// daysIn returns the number of days in a month for a given year.
// From: https://groups.google.com/forum/#!topic/golang-nuts/W-ezk71hioo
func daysIn(year int, m time.Month) int {
	// This is equivalent to time.daysIn(m, year).
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// NewPeriodFromMonth coverts a string of the form month-year into a period with the start/end of the month
func NewPeriodFromMonth(in string) (*Period, error) {
	o := strings.SplitN(in, "-", 2)
	m, err := strconv.Atoi(o[0])
	if err != nil {
		return nil, err
	}
	month := time.Month(m)
	year, err := strconv.Atoi(o[1])
	if err != nil {
		return nil, err
	}

	p := &Period{}
	p.Start = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	p.End = time.Date(year, month, daysIn(year, month), 23, 59, 59, 0, time.UTC)
	return p, nil
}

func main() {
	accessToken := flag.String("token", "", "GitHub access token")
	monthly := flag.String("monthly", "", "Month to generate the report for, e.g. 01-2018")
	verbose := flag.Int("v", 0, "Verbosity level")
	flag.Parse()

	if *accessToken == "" {
		log.Fatal("Please specify a access token")
	}
	logLevel = *verbose

	var period *Period
	if *monthly != "" {
		var err error
		if period, err = NewPeriodFromMonth(*monthly); err != nil {
			log.Fatal("Error parsing monthly", err)
		}
	}
	fmt.Printf("FROM %s TO %s\n", period.Start, period.End)

	repos := flag.Args()

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	// Phase 1: Gather information about PRs/Issues/Users

	allUsers := make(Users)
	var allPRs Items
	var allIssues Items

	for _, ownerAndRepo := range repos {
		t := strings.SplitN(ownerAndRepo, "/", 2)
		if len(t) != 2 {
			log.Printf("%s is malformed.", ownerAndRepo)
		}
		owner := t[0]
		repo := t[1]

		// Handle PRs
		infof("Handling PRs:\n")
		err := doListOp(func(page int) (*github.Response, error) {
			prOpts := &github.PullRequestListOptions{State: "all", Sort: "updated", Direction: "desc"}
			prOpts.ListOptions.Page = page
			ghPRs, resp, err := client.PullRequests.List(ctx, owner, repo, prOpts)
			if err != nil {
				log.Printf("Error getting PRs for %s: %v", repo, err)
				return nil, err
			}
			for _, ghPR := range ghPRs {
				infof("Handle PR: %s#%d %s\n", ownerAndRepo, *ghPR.Number, *ghPR.Title)
				debugf("%+v\n\n", ghPR)
				pr := NewItemFromPR(ctx, client, ghPR, ownerAndRepo, &allUsers)
				// The List options for PRs does not have a Since field (like Issues),
				// so check here when to break.
				if pr.CreatedAt.Before(period.Start) && pr.UpdatedAt.Before(period.Start) && pr.ClosedAt.Before(period.Start) {
					return nil, nil
				}
				allPRs = append(allPRs, pr)
			}
			return resp, nil
		})
		if err != nil {
			log.Printf("Error getting PRs for %s: %v", repo, err)
		}

		// Handle issues
		infof("Handling Issues:\n")
		err = doListOp(func(page int) (*github.Response, error) {
			issueOpts := &github.IssueListByRepoOptions{
				State:     "all",
				Sort:      "updated",
				Direction: "desc",
				Since:     period.Start,
			}
			issueOpts.ListOptions.Page = page
			ghIssues, resp, err := client.Issues.ListByRepo(ctx, owner, repo, issueOpts)
			if err != nil {
				log.Printf("Error getting issues for %s: %v", repo, err)
				return nil, err
			}
			for _, ghIssue := range ghIssues {
				// Only handle proper issues
				if !ghIssue.IsPullRequest() {
					infof("Handle Issue: %s#%d %s\n", ownerAndRepo, *ghIssue.Number, *ghIssue.Title)
					debugf("%+v\n\n", ghIssue)
					issue := NewItemFromIssue(ctx, client, ghIssue, ownerAndRepo, &allUsers)
					allIssues = append(allIssues, issue)
				}
			}
			return resp, nil
		})
		if err != nil {
			log.Printf("Error getting Issues for %s: %v", repo, err)
		}
	}

	// Phase 2: Filter (TODO)

	// Phase 3: Print output in markdown fragments
	fmt.Println("## PRs:")
	fmt.Println(allPRs)
	fmt.Println()
	fmt.Println("## Issues:")
	fmt.Println(allIssues)
	fmt.Println()
	fmt.Println(allPRs.Links())
	fmt.Println(allIssues.Links())
	fmt.Println(allUsers.Links())
}
