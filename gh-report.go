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

// User is a structure with information about a user
type User struct {
	ID  string
	URL string
}

// NewUser create a new User
func NewUser(u *github.User) *User {
	return &User{ID: *u.Login, URL: *u.HTMLURL}
}

func (u *User) String() string {
	return "[@" + u.ID + "]"
}

// Link returns a markdown style link to the item
func (u *User) Link() string {
	return fmt.Sprintf("[@%s]: %s", u.ID, u.URL)
}

// Users is a structure to store information about users
type Users map[string]*User

// Add adds a GH user to a map of Users if the user does not exist
func (users Users) Add(u *github.User) *User {
	if user, ok := users[*u.Login]; ok {
		return user
	}
	user := NewUser(u)
	users[user.ID] = user
	return user
}

// Links returns a string with markdown links to all users
func (users Users) Links() string {
	var ret string
	var r string
	for _, user := range users {
		r += ret + user.Link()
		if ret == "" {
			ret = "\n"
		}
	}
	return r
}

// Item has some fields extracted from GitHub issue (PRs are issues too)
type Item struct {
	issue     *github.Issue
	ID        string
	Repo      string
	Number    int
	PR        bool
	State     string
	Title     string
	URL       string
	CreatedBy *User
}

// NewItem creates an new Item and extracts some additional information
func NewItem(issue *github.Issue, repo string, users *Users) *Item {
	item := &Item{issue: issue,
		ID:     fmt.Sprintf("%s#%d", repo, *issue.Number),
		Repo:   repo,
		Number: *issue.Number,
		PR:     issue.IsPullRequest(),
		State:  *issue.State,
		Title:  *issue.Title,
		URL:    *issue.HTMLURL,
	}

	if issue.User != nil {
		item.CreatedBy = users.Add(issue.User)
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

// Items is a map of items
type Items map[string]*Item

// String return a string of a sorted list of Items in markdown
func (items Items) String() string {
	// TODO(rn): Sort by repo name and then numerically
	var keys []string
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var ret string
	var r string
	for _, item := range keys {
		r += ret + "- " + items[item].String()
		if ret == "" {
			ret = "\n"
		}
	}
	return r
}

// Links returns a string with markdown links to all issues
func (items Items) Links() string {
	var ret string
	var r string
	for _, item := range items {
		r += ret + item.Link()
		if ret == "" {
			ret = "\n"
		}
	}
	return r
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

	// Phase 1: Gather information about PRs/Issues/Users

	allUsers := make(Users)
	allPRs := make(Items)
	allIssues := make(Items)

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
			i := NewItem(issue, ownerAndRepo, &allUsers)
			if i.PR {
				allPRs[i.ID] = i
			} else {
				allIssues[i.ID] = i
			}
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
