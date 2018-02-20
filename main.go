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
	var since *time.Time
	if *monthly != "" {
		var err error
		if period, err = NewPeriodFromMonth(*monthly); err != nil {
			log.Fatal("Error parsing monthly", err)
		}
		since = &period.Start
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
		infof("Get PRs:\n")
		if err := GetPRs(ctx, client, owner, repo, since, &allPRs, &allUsers); err != nil {
			log.Printf("Error getting PRs for %s: %v", repo, err)
		}

		// Handle issues
		infof("Get Issues:\n")
		if err := GetIssues(ctx, client, owner, repo, since, &allIssues, &allUsers); err != nil {
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
