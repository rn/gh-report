package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Simple logging to stderr
var logLevel int

func warnf(format string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, format, v...)
}

func infof(format string, v ...interface{}) {
	if logLevel >= 1 {
		fmt.Fprintf(os.Stderr, format, v...)
	}
}

func debugf(format string, v ...interface{}) {
	if logLevel >= 2 {
		fmt.Fprintf(os.Stderr, format, v...)
	}
}

// Perform a GitHub API List operation considering paging and rate limiting
// If op() returns nil, we are done too
func doListOp(op func(page int) (*github.Response, error)) error {
	for page := 1; page != 0; {
		r, err := op(page)
		if err != nil {
			return err
		}
		if r == nil {
			break
		}
		infof("  doListOp: Response Page:%d Rate:%d/%d Reset:%s\n", r.NextPage, r.Limit, r.Remaining, r.Reset)

		page = r.NextPage
		// Handle rate limiting
		if r.Remaining == 0 {
			warnf("No more request this period. Limit %d reset at %s\n", r.Limit, r.Reset)
			warnf("Sleep for %s\n", time.Until(r.Reset.Time)+(5*time.Second))
			time.Sleep(time.Until(r.Reset.Time) + (5 * time.Second))
		}
	}
	return nil
}

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
	debugf("  Add user: %s\n", *u.Login)
	if user, ok := users[*u.Login]; ok {
		return user
	}
	user := NewUser(u)
	users[user.ID] = user
	infof("  Added new user: %s\n", user.ID)
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

	doListOp(func(page int) (*github.Response, error) {
		commentOpts := &github.PullRequestListCommentsOptions{}
		commentOpts.ListOptions.Page = page
		ghComments, resp, err := client.PullRequests.ListComments(ctx, t[0], t[1], i.Number, commentOpts)
		if err != nil {
			fmt.Println("Error getting comments for %s", i.ID)
			return nil, err
		}
		for _, ghComment := range ghComments {
			c := NewCommentFromPR(ghComment, users)
			i.Comments = append(i.Comments, c)
		}
		return resp, nil
	})

	doListOp(func(page int) (*github.Response, error) {
		reviewOpts := &github.ListOptions{Page: page}
		ghReviews, resp, err := client.PullRequests.ListReviews(ctx, t[0], t[1], i.Number, reviewOpts)
		if err != nil {
			fmt.Println("Error getting review comments for %s", i.ID)
			return nil, err
		}
		for _, ghReview := range ghReviews {
			c := NewCommentFromReview(ghReview, users)
			i.Comments = append(i.Comments, c)
		}
		return resp, nil
	})
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
	doListOp(func(page int) (*github.Response, error) {
		commentOpts := &github.IssueListCommentsOptions{}
		commentOpts.ListOptions.Page = page
		ghComments, resp, err := client.Issues.ListComments(ctx, t[0], t[1], i.Number, commentOpts)
		if err != nil {
			fmt.Println("Error getting comments for %s", i.ID)
			return nil, err
		}
		for _, ghComment := range ghComments {
			c := NewCommentFromIssue(ghComment, users)
			i.Comments = append(i.Comments, c)
		}
		return resp, nil
	})
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
