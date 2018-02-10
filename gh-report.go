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

// NewCommentFromPR creates a Comment from a GH pull request comment.
func NewCommentFromPR(c *github.PullRequestComment, users *Users) *Comment {
	comment := &Comment{CreatedAt: *c.CreatedAt}
	if c.User != nil {
		comment.User = users.Add(c.User)
	}
	return comment
}

// NewCommentFromReview creates a Comment from a GH pull request review.
func NewCommentFromReview(c *github.PullRequestReview, users *Users) *Comment {
	comment := &Comment{CreatedAt: *c.SubmittedAt}
	if c.User != nil {
		comment.User = users.Add(c.User)
	}
	return comment
}

// NewCommentFromIssue creates a Comment from a GH issue comment.
func NewCommentFromIssue(c *github.IssueComment, users *Users) *Comment {
	comment := &Comment{CreatedAt: *c.CreatedAt}
	if c.User != nil {
		comment.User = users.Add(c.User)
	}
	return comment
}

// Item is either a Issue or a PR
type Item struct {
	PR        bool
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
	// PR specific fields
	Merged   bool
	MergedBy *User
}

// NewItemFromPR creates an new Item and extracts some additional information
func NewItemFromPR(ctx context.Context, client *github.Client, pr *github.PullRequest, repo string, users *Users) *Item {
	i := &Item{PR: true,
		ID:        fmt.Sprintf("%s#%d", repo, *pr.Number),
		Repo:      repo,
		Number:    *pr.Number,
		State:     *pr.State,
		Title:     *pr.Title,
		URL:       *pr.HTMLURL,
		CreatedAt: *pr.CreatedAt,
	}

	if pr.User != nil {
		i.CreatedBy = users.Add(pr.User)
	}
	if pr.MergedBy != nil {
		i.MergedBy = users.Add(pr.MergedBy)
	}
	if pr.UpdatedAt != nil {
		i.UpdatedAt = *pr.UpdatedAt
	}
	if pr.ClosedAt != nil {
		i.ClosedAt = *pr.ClosedAt
	}
	if pr.Merged != nil {
		i.Merged = *pr.Merged
	}

	t := strings.SplitN(repo, "/", 2)

	ghComments, _, err := client.PullRequests.ListComments(ctx, t[0], t[1], i.Number, nil)
	if err != nil {
		fmt.Println("Error getting comments for %s", i.ID)
	} else {
		for _, ghComment := range ghComments {
			c := NewCommentFromPR(ghComment, users)
			i.Comments = append(i.Comments, c)
		}
	}

	ghReviews, _, err := client.PullRequests.ListReviews(ctx, t[0], t[1], i.Number, nil)
	if err != nil {
		fmt.Println("Error getting review comments for %s", i.ID)
	} else {
		for _, ghReview := range ghReviews {
			c := NewCommentFromReview(ghReview, users)
			i.Comments = append(i.Comments, c)
		}
	}
	return i
}

// NewItemFromIssue creates an new Item and extracts some additional information
func NewItemFromIssue(ctx context.Context, client *github.Client, issue *github.Issue, repo string, users *Users) *Item {
	i := &Item{PR: false,
		ID:        fmt.Sprintf("%s#%d", repo, *issue.Number),
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

	t := strings.SplitN(repo, "/", 2)
	ghComments, _, err := client.Issues.ListComments(ctx, t[0], t[1], i.Number, nil)
	if err != nil {
		fmt.Println("Error getting comments for %s", i.ID)
	} else {
		for _, ghComment := range ghComments {
			c := NewCommentFromIssue(ghComment, users)
			i.Comments = append(i.Comments, c)
		}
	}
	return i
}

func (i *Item) String() string {
	ret := fmt.Sprintf("%s ([%s] %s", i.Title, i.ID, i.CreatedBy)

	if i.MergedBy != nil {
		ret += " " + i.MergedBy.String()
	}

	// Make the list of contributors unique
	set := make(map[string]struct{})
	for _, c := range i.Comments {
		if c.User != i.CreatedBy && c.User != i.MergedBy {
			set[c.User.String()] = struct{}{}
		}
	}
	for u := range set {
		ret += " " + u
	}
	return ret + ")"
}

// Link returns a markdown style link to the issue
func (i *Item) Link() string {
	return fmt.Sprintf("[%s]: %s", i.ID, i.URL)
}

// Items is list of items (Issues & PRs)
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

// Links returns a string with markdown links to all items
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

		// Handle PRs
		prOpts := &github.PullRequestListOptions{State: "all"}
		ghPRs, _, err := client.PullRequests.List(ctx, owner, repo, prOpts)
		if err != nil {
			log.Printf("Error getting PRs for %s: %v", repo, err)
			continue
		}
		for _, ghPR := range ghPRs {
			pr := NewItemFromPR(ctx, client, ghPR, ownerAndRepo, &allUsers)
			allPRs = append(allPRs, pr)
		}

		// Handle issues
		issueOpts := &github.IssueListByRepoOptions{State: "all"}
		ghIssues, _, err := client.Issues.ListByRepo(ctx, owner, repo, issueOpts)
		if err != nil {
			log.Printf("Error getting issues for %s: %v", repo, err)
			continue
		}
		for _, ghIssue := range ghIssues {
			// Only handle proper issues
			if !ghIssue.IsPullRequest() {
				issue := NewItemFromIssue(ctx, client, ghIssue, ownerAndRepo, &allUsers)
				allIssues = append(allIssues, issue)
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
