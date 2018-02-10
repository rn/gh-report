package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

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

// Comment represents a Comment on a Issue or PR
type Comment struct {
	CreatedAt time.Time
	User      *User
}

// NewComment creates a Comment from a GH comment.
func NewComment(c *github.IssueComment, users *Users) *Comment {
	comment := &Comment{CreatedAt: *c.CreatedAt}
	if c.User != nil {
		comment.User = users.Add(c.User)
	}
	return comment
}

// Item has some fields extracted from GitHub issue (PRs are issues too)
type Item struct {
	ID        string
	Repo      string
	Number    int
	PR        bool
	State     string
	Title     string
	URL       string
	CreatedBy *User
	CreatedAt time.Time
	UpdatedAt time.Time
	ClosedAt  time.Time
	Comments  []*Comment
}

// NewItem creates an new Item and extracts some additional information
func NewItem(ctx context.Context, client *github.Client, issue *github.Issue, repo string, users *Users) *Item {
	item := &Item{ID: fmt.Sprintf("%s#%d", repo, *issue.Number),
		Repo:      repo,
		Number:    *issue.Number,
		PR:        issue.IsPullRequest(),
		State:     *issue.State,
		Title:     *issue.Title,
		URL:       *issue.HTMLURL,
		CreatedAt: *issue.CreatedAt,
	}

	if issue.User != nil {
		item.CreatedBy = users.Add(issue.User)
	}
	if issue.UpdatedAt != nil {
		item.UpdatedAt = *issue.UpdatedAt
	}
	if issue.ClosedAt != nil {
		item.ClosedAt = *issue.ClosedAt
	}

	if *issue.Comments != 0 {
		t := strings.SplitN(repo, "/", 2)
		ghcomments, _, err := client.Issues.ListComments(ctx, t[0], t[1], item.Number, nil)
		if err != nil {
			fmt.Println("Error getting comments for %s", item.ID)
		} else {
			for _, comment := range ghcomments {
				c := NewComment(comment, users)
				item.Comments = append(item.Comments, c)
			}
		}
	}
	return item
}

func (i *Item) String() string {
	ret := fmt.Sprintf("%s ([%s] %s", i.Title, i.ID, i.CreatedBy)

	// Make the list of contributors unique
	set := make(map[string]struct{})
	for _, c := range i.Comments {
		if c.User != i.CreatedBy {
			set[c.User.String()] = struct{}{}
		}
	}
	for u := range set {
		ret += " " + u
	}
	return ret + ")"
}

// Link returns a markdown style link to the item
func (i *Item) Link() string {
	return fmt.Sprintf("[%s]: %s", i.ID, i.URL)
}

// Items is a map of items
type Items []*Item

// String return a string of a sorted list of Items in markdown
func (items Items) String() string {
	// Sort slice: If the items are from the same repo, use the number otherwise use the repo name.
	sort.Slice(items, func(i, j int) bool {
		if items[i].Repo != items[j].Repo {
			return items[i].Repo < items[j].Repo
		}
		return items[i].Number < items[j].Number
	})
	var ret string
	var r string
	for _, item := range items {
		r += ret + "- " + item.String()
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
	var allPRs Items
	var allIssues Items

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
			i := NewItem(ctx, client, issue, ownerAndRepo, &allUsers)
			if i.PR {
				allPRs = append(allPRs, i)
			} else {
				allIssues = append(allIssues, i)
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
