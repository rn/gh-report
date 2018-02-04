package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Item has some fields extracted from GitHub issue (PRs are issues too)
type Item struct {
	issue     *github.Issue
	ID        string
	PR        bool
	State     string
	Title     string
	URL       string
	CreatedBy *User
}

func newItem(issue *github.Issue, repo string) *Item {
	item := &Item{issue: issue,
		ID:    fmt.Sprintf("%s#%d", repo, *issue.Number),
		PR:    issue.IsPullRequest(),
		State: *issue.State,
		Title: *issue.Title,
		URL:   *issue.HTMLURL,
	}

	if issue.User != nil {
		item.CreatedBy = newUser(issue.User)
	}
	return item
}

func (i *Item) String() string {
	return fmt.Sprintf("%s ([%s] %s)", i.Title, i.ID, i.CreatedBy)
}

// Link returns a markdown style link to the item
func (i *Item) Link() string {
	return fmt.Sprintf("[%s]: %s", i.ID, i.URL)
}

// User is a structure with information about a user
type User struct {
	ID  string
	URL string
}

var users map[string]*User

// Create a new User if it is not in the map
func newUser(u *github.User) *User {
	if user, ok := users[*u.Login]; ok {
		return user
	}
	user := &User{ID: *u.Login, URL: *u.HTMLURL}
	users[user.ID] = user
	return user
}

func (u *User) String() string {
	return "[@" + u.ID + "]"
}

// Link returns a markdown style link to the item
func (u *User) Link() string {
	return fmt.Sprintf("[@%s]: %s", u.ID, u.URL)
}

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

	users = make(map[string]*User)
	prs := make(map[string]*Item)
	issues := make(map[string]*Item)

	for _, ownerAndRepo := range repos {
		t := strings.SplitN(ownerAndRepo, "/", 2)
		if len(t) != 2 {
			log.Printf("%s is malformed.", ownerAndRepo)
		}
		owner := t[0]
		repo := t[1]

		listOpts := &github.IssueListByRepoOptions{State: "all"}
		ghissues, _, err := client.Issues.ListByRepo(ctx, owner, repo, listOpts)
		if err != nil {
			log.Printf("Error getting issues for %s: %v", repo, err)
			continue
		}
		for _, issue := range ghissues {
			i := newItem(issue, ownerAndRepo)
			// Only report on closed issues/PRs
			if i.State != "closed" {
				continue
			}
			if i.PR {
				prs[i.ID] = i
			} else {
				issues[i.ID] = i
			}
		}
	}

	// Print output in markdown fragments
	fmt.Println()
	fmt.Println("## PRs:")
	var prkeys []string
	for k := range prs {
		prkeys = append(prkeys, k)
	}
	sort.Strings(prkeys)
	for _, pr := range prkeys {
		fmt.Printf("- %s\n", prs[pr])
	}

	fmt.Println()
	fmt.Println("## Issues:")
	var issuekeys []string
	for k := range issues {
		issuekeys = append(issuekeys, k)
	}
	sort.Strings(issuekeys)
	for _, issue := range issuekeys {
		fmt.Printf("- %s\n", issues[issue])
	}

	fmt.Println()
	for _, pr := range prs {
		fmt.Println(pr.Link())
	}
	for _, issue := range issues {
		fmt.Println(issue.Link())
	}
	for _, user := range users {
		fmt.Println(user.Link())
	}
}
