package main

import (
	"fmt"
	"os"
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
