package main

import (
	"fmt"
	"os"
)

// stdPrinter provides Printf() to console
type stdPrinter struct{}

// Printf is a wrapper for fmt.Printf() and prints to stdout
func (cp stdPrinter) Printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf(format, a...)
}

// ErrPrintf is a wrapper for fmt.Printf() and prints to stderr
func (cp stdPrinter) ErrPrintf(format string, a ...interface{}) (n int, err error) {
	return fmt.Fprintf(os.Stderr, format, a...)
}
