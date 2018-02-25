# `gh-report`

A small utility to generate weekly or monthly reports for GitHub
repositories as mark-down.

For example:

```sh
gh-report -token <github token> -weekly 2018-8 linuxkit/linuxkit linuxkit/kubernetes
```

generates a report of activity on two repositories for ISO week 8
in 2018. It prints a small summary of contributions (PRs, Issues,
comments), number of individual contributors, merged and creates PRs
and Issues all with hyperlinks.

It can also be used to generate activity reports for an individual
user by specifying the `-user` option.
