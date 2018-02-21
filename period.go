package main

import (
	"strconv"
	"strings"
	"time"
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
	year, err := strconv.Atoi(o[0])
	if err != nil {
		return nil, err
	}
	m, err := strconv.Atoi(o[1])
	if err != nil {
		return nil, err
	}
	month := time.Month(m)

	p := &Period{}
	p.Start = time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	p.End = time.Date(year, month, daysIn(year, month), 23, 59, 59, 0, time.UTC)
	return p, nil
}

// Return the monday of the ISO week in the given year
// From: https://play.golang.org/p/UVFNFcpaoI
func firstDayOfISOWeek(year int, week int) time.Time {
	date := time.Date(year, 0, 0, 0, 0, 0, 0, time.UTC)
	isoYear, isoWeek := date.ISOWeek()
	for date.Weekday() != time.Monday { // iterate back to Monday
		date = date.AddDate(0, 0, -1)
		isoYear, isoWeek = date.ISOWeek()
	}
	for isoYear < year { // iterate forward to the first day of the first week
		date = date.AddDate(0, 0, 1)
		isoYear, isoWeek = date.ISOWeek()
	}
	for isoWeek < week { // iterate forward to the first day of the given week
		date = date.AddDate(0, 0, 1)
		isoYear, isoWeek = date.ISOWeek()
	}
	return date
}

// NewPeriodFromWeek coverts a string of the form week-year into a period with the start/end of the month
func NewPeriodFromWeek(in string) (*Period, error) {
	o := strings.SplitN(in, "-", 2)
	year, err := strconv.Atoi(o[0])
	if err != nil {
		return nil, err
	}
	week, err := strconv.Atoi(o[1])
	if err != nil {
		return nil, err
	}

	p := &Period{}
	p.Start = firstDayOfISOWeek(year, week)
	p.End = p.Start.AddDate(0, 0, 7)
	return p, nil
}

// Match returns true if t falls within the period
func (p *Period) Match(t time.Time) bool {
	return t.After(p.Start) && t.Before(p.End)
}
