package app

import (
	"testing"
	"time"

	"bharvest.io/axelmon/client/api"
)

func TestCountVotesAndLegacy(t *testing.T) {
	now := time.Now()
	records := []api.Record{
		// Everybody voted no and the poll failed: agreement.
		{ID: "1", Created: now, Failed: true, Participants: []string{"axelar1a", "axelar1b", "axelar1c"},
			Entries: map[string]api.Entry{"axelar1a": {}, "axelar1b": {}, "axelar1c": {}}},
		// We voted no against a yes majority.
		{ID: "2", Created: now, Success: true, Participants: []string{"axelar1a", "axelar1b", "axelar1c"},
			Entries: map[string]api.Entry{"axelar1a": {}, "axelar1b": {Value: true}, "axelar1c": {Value: true}}},
		// We did not vote.
		{ID: "3", Created: now, Success: true, Participants: []string{"axelar1a", "axelar1b", "axelar1c"},
			Entries: map[string]api.Entry{"axelar1b": {Value: true}, "axelar1c": {Value: true}}},
		// Still open: not counted.
		{ID: "4", Created: now, Participants: []string{"axelar1a", "axelar1b", "axelar1c"},
			Entries: map[string]api.Entry{"axelar1b": {Value: true}}},
	}

	counts := countVotes(records, "axelar1a", "axelar1a")
	want := map[api.VoteResult]int{api.VoteAgreed: 1, api.VoteMinority: 1, api.VoteNotVoted: 1}
	for _, r := range api.VoteResults {
		if counts[r] != want[r] {
			t.Errorf("%s: got %d, want %d", r, counts[r], want[r])
		}
	}

	// The legacy count calls both "no" votes missed and cannot see the absent vote.
	legacy := legacyVotes(records)["axelar1a"]
	if legacy.MissedCnt != 2 || legacy.TotalCnt != 2 {
		t.Errorf("legacy: got %d / %d, want 2 / 2", legacy.MissedCnt, legacy.TotalCnt)
	}
}

func TestCheckWindow(t *testing.T) {
	c := &Config{}
	if got := c.checkWindow(); got != 10*24*time.Hour {
		t.Errorf("default: got %v", got)
	}
	c.PollingVote.CheckPeriodDays = 1
	if got := c.checkWindow(); got != 24*time.Hour {
		t.Errorf("days: got %v", got)
	}
	c.PollingVote.CheckPeriodHours = 6
	if got := c.checkWindow(); got != 6*time.Hour {
		t.Errorf("hours: got %v", got)
	}
}
