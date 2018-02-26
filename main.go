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
	user := flag.String("user", "", "Only report activity for a single user")
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

	// Gather information about PRs/Issues/Users

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

	if *user != "" {
		userReport(repos, period, *user, allPRs, allIssues)
	} else {
		repoReport(repos, period, allPRs, allIssues)
	}
}

// repoReport generates a report about activity on Repositories
func repoReport(repos []string, period *Period, allPRs, allIssues Items) {
	var mergedPRs Items
	var closedIssues Items
	var updatedItems Items

	var openedPRsCount int
	var openedIssuesCount int
	var contributions int
	contributors := make(Users)
	users := make(Users)

	for _, i := range append(allPRs, allIssues...) {
		infof("Processing: %s\n", i)
		debugf("%s\n\n", i.Dump())

		var updated bool
		
		// Record all users as they may be linked in Issues/PRs
		users[i.CreatedBy.ID] = i.CreatedBy

		// Handle comments first
		for _, comment := range i.Comments {
			if period.Match(comment.CreatedAt) {
				contributions++
				contributors[comment.User.ID] = comment.User
				updated = true
			}
			// Record all users as they may be linked in Issues/PRs
			users[comment.User.ID] = comment.User
		}

		// Next handle contributions from new Items
		if period.Match(i.CreatedAt) {
			updated = true
			// Only count contributor here. Item will be put on the appropriate list below
			contributions++
			contributors[i.CreatedBy.ID] = i.CreatedBy
			if i.PR {
				openedPRsCount++
			} else {
				openedIssuesCount++
			}
		}

		// Next handle closed PRs and issues
		if period.Match(i.ClosedAt) {
			if i.PR {
				if i.Merged {
					mergedPRs = append(mergedPRs, i)
					// Sigh...sometimes MergedBy is not filled in
					if i.MergedBy != nil {
						users[i.MergedBy.ID] = i.MergedBy
						contributors[i.MergedBy.ID] = i.MergedBy
					}
				} else {
					// PR was *not* merged. Count as updated
					updatedItems = append(updatedItems, i)
				}
			} else {
				// Issues, just add to closed issues list
				closedIssues = append(closedIssues, i)
			}
		} else {
			if updated {
				// Not closed, but updated, so add to updated list
				// Contributions were already counted.
				updatedItems = append(updatedItems, i)
			}
		}
	}

	// Print output in markdown fragments

	fmt.Printf("# Report for %s\n", *period)
	fmt.Println()
	fmt.Printf("This report covers the development in the")
	for _, ownerAndRepo := range repos {
		fmt.Printf(" [%s]", ownerAndRepo)
	}
	fmt.Printf(" repositories. There were %d contributions (PRs/Issues/Comments) from %d individual contributors. %d new PRs were opened and %d PRs were merged. %d new issues were opened and %d issues were closed.\n", contributions, len(contributors), openedPRsCount, len(mergedPRs), openedIssuesCount, len(closedIssues))
	fmt.Println()

	// Details
	fmt.Println("## Merged PRs:")
	fmt.Println(mergedPRs)
	fmt.Println()
	fmt.Println("## Closed Issues:")
	fmt.Println(closedIssues)
	fmt.Println()
	fmt.Println("## New or updated PRs and Issues (not closed):")
	fmt.Println(updatedItems)

	// Links
	fmt.Println()
	for _, ownerAndRepo := range repos {
		fmt.Printf("[%s]: https://github.com/%s\n", ownerAndRepo, ownerAndRepo)
	}
	fmt.Println(mergedPRs.Links())
	fmt.Println(closedIssues.Links())
	fmt.Println(updatedItems.Links())
	fmt.Println(users.Links())
}

// userReport generates a report about activity of a single user
func userReport(repos []string, period *Period, user string, allPRs, allIssues Items) {
	var userPRs Items
	var reviewedPRs Items
	var userIssues Items
	var commentIssues Items

	for _, pr := range allPRs {
		if period.Match(pr.CreatedAt) && pr.CreatedBy.ID == user {
			userPRs = append(userPRs, pr)
			continue
		}
		if period.Match(pr.ClosedAt) && pr.MergedBy != nil && pr.MergedBy.ID == user {
			reviewedPRs = append(reviewedPRs, pr)
			continue
		}
		for _, comment := range pr.Comments {
			if period.Match(comment.CreatedAt) && comment.User.ID == user {
				reviewedPRs = append(reviewedPRs, pr)
				break
			}
		}
	}
	for _, i := range allIssues {
		if period.Match(i.CreatedAt) && i.CreatedBy.ID == user {
			userIssues = append(userIssues, i)
			continue
		}
		for _, comment := range i.Comments {
			if period.Match(comment.CreatedAt) && comment.User.ID == user {
				commentIssues = append(commentIssues, i)
				break
			}
		}
	}

	fmt.Println("## PRs:")
	fmt.Println(userPRs)
	fmt.Println()
	fmt.Println("## Reviewed PRs:")
	fmt.Println(reviewedPRs)
	fmt.Println()
	fmt.Println("## Issues:")
	fmt.Println(userIssues)
	fmt.Println()
	fmt.Println("## Issues commented on:")
	fmt.Println(commentIssues)
	fmt.Println()
	fmt.Println(userPRs.Links())
	fmt.Println(reviewedPRs.Links())
	fmt.Println(userIssues.Links())
	fmt.Println(commentIssues.Links())
}
