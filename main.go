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
	monthly := flag.String("monthly", "", "Month to generate the report for, e.g. 2018-01")
	weekly := flag.String("weekly", "", "(ISO) week to generate the report for, e.g. 2018-01")
	verbose := flag.Int("v", 0, "Verbosity level")
	flag.Parse()

	if *accessToken == "" {
		log.Fatal("Please specify a access token")
	}
	logLevel = *verbose

	if (*monthly == "" && *weekly == "") || (*monthly != "" && *weekly != "") {
		log.Fatal("Please specify either a month or a week")
	}
	var err error
	var period *Period
	if *monthly != "" {
		period, err = NewPeriodFromMonth(*monthly)
		if err != nil {
			log.Fatal("Error parsing month:", err)
		}
	}
	if *weekly != "" {
		period, err = NewPeriodFromWeek(*weekly)
		if err != nil {
			log.Fatal("Error parsing week:", err)
		}
	}
	infof("FROM %s TO %s\n", period.Start, period.End)

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
		infof("Get PRs:\n")
		if err := GetPRs(ctx, client, owner, repo, &period.Start, &allPRs, &allUsers); err != nil {
			log.Printf("Error getting PRs for %s: %v", repo, err)
		}

		// Handle issues
		infof("Get Issues:\n")
		if err := GetIssues(ctx, client, owner, repo, &period.Start, &allIssues, &allUsers); err != nil {
			log.Printf("Error getting Issues for %s: %v", repo, err)
		}
	}

	// Phase 2: Filter

	var mergedPRs Items
	var openedPRs Items
	var closedIssues Items
	var openedIssuesCount int
	var openedIssues Items
	var contributions int
	contributors := make(Users)
	users := make(Users)

	for _, i := range append(allPRs, allIssues...) {
		infof("Processing: %s\n", i)
		debugf("%s\n\n", i.Dump())
		users[i.CreatedBy.ID] = i.CreatedBy
		if period.Match(i.CreatedAt) {
			contributions++
			contributors[i.CreatedBy.ID] = i.CreatedBy
		}
		for _, comment := range i.Comments {
			if period.Match(comment.CreatedAt) {
				contributions++
				contributors[i.CreatedBy.ID] = i.CreatedBy
			}
			users[comment.User.ID] = comment.User
		}

		if i.PR {
			if period.Match(i.CreatedAt) {
				openedPRs = append(openedPRs, i)
			}
			if period.Match(i.ClosedAt) && i.Merged {
				mergedPRs = append(mergedPRs, i)
				contributions++
				// Sigh...sometimes MergedBy is not filled in
				if i.MergedBy != nil {
					users[i.MergedBy.ID] = i.MergedBy
					contributors[i.MergedBy.ID] = i.MergedBy
				}
			}
		} else {
			if period.Match(i.CreatedAt) {
				openedIssuesCount++
				if !period.Match(i.ClosedAt) {
					openedIssues = append(openedIssues, i)
				}
			}
			if period.Match(i.ClosedAt) {
				closedIssues = append(closedIssues, i)
			}
		}
	}

	// Phase 3: Print output in markdown fragments

	fmt.Printf("# Report for %s\n", *monthly)

	fmt.Printf("This report covers the following repositories:")
	for _, ownerAndRepo := range repos {
		fmt.Printf(" [%s]", ownerAndRepo)
	}
	fmt.Println()
	fmt.Printf("In the reporting period there were %d contributions (PRs/Issues/Comments) from %d individual contributors. %d new PRs were opened and %d PRs were merged. %d issues were opened and %d issues were closed.\n", contributions, len(contributors), len(openedPRs), len(mergedPRs), openedIssuesCount, len(closedIssues))
	fmt.Println()

	fmt.Println("## Merged PRs:")
	fmt.Println(mergedPRs)
	fmt.Println()
	fmt.Println("## Closed Issues:")
	fmt.Println(closedIssues)
	fmt.Println()
	fmt.Println("## Opened Issues (still open):")
	fmt.Println(openedIssues)
	fmt.Println()
	for _, ownerAndRepo := range repos {
		fmt.Printf("[%s]: https://github.com/%s\n", ownerAndRepo, ownerAndRepo)
	}
	fmt.Println(mergedPRs.Links())
	fmt.Println(closedIssues.Links())
	fmt.Println(openedIssues.Links())
	fmt.Println(users.Links())
}
