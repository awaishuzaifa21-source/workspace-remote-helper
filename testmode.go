package main

import (
	"encoding/json"
	"os"
)

// Test mode (WSH_TEST=1) records events to WSH_LOG instead of moving anything, and answers
// the consent question itself (WSH_DECLINE=1 says No). Used by the automated browser tests.
var testMode = os.Getenv("WSH_TEST") == "1"

func testConfirm() bool { return os.Getenv("WSH_DECLINE") != "1" }

func testLog(e event) {
	if p := os.Getenv("WSH_LOG"); p != "" {
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err == nil {
			json.NewEncoder(f).Encode(e)
			f.Close()
		}
	}
}

// clean01 keeps a screen fraction inside 0..1.
func clean01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// wheelSteps turns pixels of scrolling into wheel clicks, at most 10 per event.
func wheelSteps(dy float64) int {
	n := int((abs64(dy) + 50) / 100)
	if n < 1 {
		n = 1
	}
	if n > 10 {
		n = 10
	}
	return n
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
