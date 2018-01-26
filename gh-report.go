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
	flag.Parse()

	if *accessToken == "" {
		log.Fatal("Please specify a access token")
	}

	repos := flag.Args()

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: *accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	for _, owner_repo := range repos {
		t := strings.SplitN(owner_repo, "/", 2)
		if len(t) != 2 {
			log.Printf("%s is malformed.", owner_repo)
		}
		owner := t[0]
		repo := t[1]

		listOpts := &github.IssueListByRepoOptions{State: "all"}
		issues, _, err := client.Issues.ListByRepo(ctx, owner, repo, listOpts)
		if err != nil {
			log.Printf("Error getting issues for %s: %v", repo, err)
			continue
		}
		for _, issue := range issues {
			fmt.Printf("#%d(%s): %s\n", *issue.Number, *issue.State, *issue.Title)
			fmt.Println(issue)
			fmt.Println()
		}
	}
}
