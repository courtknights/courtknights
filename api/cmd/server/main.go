// Package main is the entry point for the CourtKnights API server.
package main

import "log"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		log.Fatal(err)
	}
}
