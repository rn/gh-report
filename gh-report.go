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

// Link returns a markdown style link to the user
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

// NewComment creates a Comment from a GH issue comment.
func NewComment(c *github.IssueComment, users *Users) *Comment {
	comment := &Comment{CreatedAt: *c.CreatedAt}
	if c.User != nil {
		comment.User = users.Add(c.User)
	}
	return comment
}

// Issue has some fields extracted from GitHub issue
type Issue struct {
	ID        string
	Repo      string
	Number    int
	State     string
	Title     string
	URL       string
	CreatedBy *User
	CreatedAt time.Time
	UpdatedAt time.Time
	ClosedAt  time.Time
	Comments  []*Comment
}

// NewIssue creates an new Issue and extracts some additional information
func NewIssue(ctx context.Context, client *github.Client, issue *github.Issue, repo string, users *Users) *Issue {
	i := &Issue{ID: fmt.Sprintf("%s#%d", repo, *issue.Number),
		Repo:      repo,
		Number:    *issue.Number,
		State:     *issue.State,
		Title:     *issue.Title,
		URL:       *issue.HTMLURL,
		CreatedAt: *issue.CreatedAt,
	}

	if issue.User != nil {
		i.CreatedBy = users.Add(issue.User)
	}
	if issue.UpdatedAt != nil {
		i.UpdatedAt = *issue.UpdatedAt
	}
	if issue.ClosedAt != nil {
		i.ClosedAt = *issue.ClosedAt
	}

	if *issue.Comments != 0 {
		t := strings.SplitN(repo, "/", 2)
		ghcomments, _, err := client.Issues.ListComments(ctx, t[0], t[1], i.Number, nil)
		if err != nil {
			fmt.Println("Error getting comments for %s", issue.ID)
		} else {
			for _, comment := range ghcomments {
				c := NewComment(comment, users)
				i.Comments = append(i.Comments, c)
			}
		}
	}
	return i
}

func (i *Issue) String() string {
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

// Link returns a markdown style link to the issue
func (i *Issue) Link() string {
	return fmt.Sprintf("[%s]: %s", i.ID, i.URL)
}

// Issues is a map of issues
type Issues []*Issue

// String return a string of a sorted list of Issues in markdown
func (issues Issues) String() string {
	// Sort slice: If the issues are from the same repo, use the number otherwise use the repo name.
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Repo != issues[j].Repo {
			return issues[i].Repo < issues[j].Repo
		}
		return issues[i].Number < issues[j].Number
	})
	var ret string
	var r string
	for _, issue := range issues {
		r += ret + "- " + issue.String()
		if ret == "" {
			ret = "\n"
		}
	}
	return r
}

// Links returns a string with markdown links to all issues
func (issues Issues) Links() string {
	var ret string
	var r string
	for _, issue := range issues {
		r += ret + issue.Link()
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
	var allIssues Issues

	for _, ownerAndRepo := range repos {
		t := strings.SplitN(ownerAndRepo, "/", 2)
		if len(t) != 2 {
			log.Printf("%s is malformed.", ownerAndRepo)
		}
		owner := t[0]
		repo := t[1]

		// Handle issues
		listOpts := &github.IssueListByRepoOptions{State: "all"}
		ghissues, _, err := client.Issues.ListByRepo(ctx, owner, repo, listOpts)
		if err != nil {
			log.Printf("Error getting issues for %s: %v", repo, err)
			continue
		}
		for _, issue := range ghissues {
			// Only handle proper issues
			if !issue.IsPullRequest() {
				i := NewIssue(ctx, client, issue, ownerAndRepo, &allUsers)
				allIssues = append(allIssues, i)
			}
		}
	}

	// Phase 2: Filter (TODO)

	// Phase 3: Print output in markdown fragments
	fmt.Println("## Issues:")
	fmt.Println(allIssues)
	fmt.Println()
	fmt.Println(allIssues.Links())
	fmt.Println(allUsers.Links())
}
