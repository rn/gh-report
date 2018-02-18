package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func main() {
	accessToken := flag.String("token", "", "GitHub access token")
	numItems := flag.Int("i", 50, "Number of PRs and Issues to procees")
	verbose := flag.Int("v", 0, "Verbosity level")
	flag.Parse()

	if *accessToken == "" {
		log.Fatal("Please specify a access token")
	}
	logLevel = *verbose

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
				allPRs = append(allPRs, pr)
				// XXX Temporary limit
				if len(allPRs) >= *numItems {
					return nil, nil
				}
			}
			return resp, nil
		})
		if err != nil {
			log.Printf("Error getting PRs for %s: %v", repo, err)
		}

		// Handle issues
		infof("Handling Issues:\n")
		err = doListOp(func(page int) (*github.Response, error) {
			issueOpts := &github.IssueListByRepoOptions{State: "all", Sort: "updated", Direction: "desc"}
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
					// XXX Temporary limit
					if len(allIssues) >= *numItems {
						return nil, nil
					}
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
