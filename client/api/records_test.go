package api

import (
	"os"
	"testing"
	"time"
)

const (
	// Real mainnet accounts from the records in testdata.
	ourVerifier    = "axelar1g92hckcernmgm60tm527njl6j2cxysm7zg6ulk"
	noVoterIn4642  = "axelar1zhazt54ewqhva5pujhfyhr7sf39hm7myatmjtd"
	absentVerifier = "axelar1kaeq00sgqvy65sngedc8dqwxerqzsg2xf7e72z"
	ourValidator   = "axelar1y6e8xuhqjdujx4jlt8lcpdzrtzcqxdqndy2t4a"
	stranger       = "axelar1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq"
)

func load(t *testing.T, file string, key ValueKey) []Record {
	t.Helper()
	body, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	records, err := ParseRecords(body, key)
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func byID(t *testing.T, records []Record, suffix string) Record {
	t.Helper()
	for _, r := range records {
		if len(r.ID) >= len(suffix) && r.ID[len(r.ID)-len(suffix):] == suffix {
			return r
		}
	}
	t.Fatalf("no record with id suffix %s", suffix)
	return Record{}
}

func expectVote(t *testing.T, name string, r Record, voter string, want VoteResult, wantCounted bool, ids ...string) {
	t.Helper()
	got, counted := ClassifyVote(r, voter, ids...)
	if got != want || counted != wantCounted {
		t.Errorf("%s: got (%q, %v), want (%q, %v)", name, got, counted, want, wantCounted)
	}
}

// Stellar poll 4644: all 24 voters of 26 participants voted "no". The poll
// "failed", and every "no" voter agreed with the majority.
func TestVerifierPollWithNoMajority(t *testing.T) {
	r := byID(t, load(t, "vm_polls.json", VoteValue), "_4644")
	if len(r.Participants) != 26 || len(r.Entries) != 24 || !r.Failed {
		t.Fatalf("unexpected record: %d participants, %d entries, failed=%v", len(r.Participants), len(r.Entries), r.Failed)
	}
	if yes, decided := r.Majority(); yes || !decided {
		t.Fatalf("majority: got (yes=%v, decided=%v), want a decided no", yes, decided)
	}
	expectVote(t, "no vote with a no majority", r, ourVerifier, VoteAgreed, true, ourVerifier)
	expectVote(t, "participant without a vote", r, absentVerifier, VoteNotVoted, true, absentVerifier)
	expectVote(t, "account outside the poll", r, stranger, "", false, stranger)
}

// Stellar poll 4642: 22 "yes" and 2 "no" of 26 participants. The poll succeeded.
func TestVerifierPollWithYesMajority(t *testing.T) {
	r := byID(t, load(t, "vm_polls.json", VoteValue), "_4642")
	if yes, decided := r.Majority(); !yes || !decided {
		t.Fatalf("majority: got (yes=%v, decided=%v), want a decided yes", yes, decided)
	}
	expectVote(t, "yes vote with a yes majority", r, ourVerifier, VoteAgreed, true, ourVerifier)
	expectVote(t, "no vote against a yes majority", r, noVoterIn4642, VoteMinority, true, noVoterIn4642)
	expectVote(t, "participant without a vote", r, absentVerifier, VoteNotVoted, true, absentVerifier)
}

// A validator votes with its proxy account, and the participant list names the
// validator by its axelarvaloper address.
func TestValidatorPoll(t *testing.T) {
	r := load(t, "evm_polls.json", VoteValue)[0]
	if len(r.Participants) == 0 || len(r.Entries) == 0 || !r.Success {
		t.Fatalf("unexpected record: %d participants, %d entries, success=%v", len(r.Participants), len(r.Entries), r.Success)
	}
	for voter := range r.Entries {
		if len(voter) < 7 || voter[:7] != "axelar1" {
			t.Fatalf("entry key %q is not an account", voter)
		}
	}
	expectVote(t, "our validator", r, ourValidator, VoteAgreed, true, "axelarvaloper1unknown")

	// A participant (by valoper address) whose proxy account has no entry.
	expectVote(t, "validator without a vote", r, stranger, VoteNotVoted, true, r.Participants[0])
	expectVote(t, "account outside the poll", r, stranger, "", false, "axelarvaloper1unknown")
}

func poll(participants int, yes, no int, mutate func(*Record)) Record {
	r := Record{ID: "t", Created: time.Now(), Entries: map[string]Entry{}}
	for i := 0; i < participants; i++ {
		r.Participants = append(r.Participants, name(i))
	}
	for i := 0; i < yes; i++ {
		r.Entries[name(i)] = Entry{Value: true}
	}
	for i := yes; i < yes+no; i++ {
		r.Entries[name(i)] = Entry{Value: false}
	}
	if mutate != nil {
		mutate(&r)
	}
	return r
}

func name(i int) string { return "axelar1p" + string(rune('a'+i)) }

func TestClassifyVoteEdges(t *testing.T) {
	// 10 participants: 0..5 vote yes, 6..7 vote no, 8..9 do not vote.
	yesVoter, noVoter, silent := name(0), name(6), name(9)

	open := poll(10, 6, 2, nil)
	expectVote(t, "open poll, voted", open, yesVoter, "", false, yesVoter)
	expectVote(t, "open poll, not voted yet", open, silent, "", false, silent)

	late := poll(10, 6, 2, func(r *Record) {
		r.Success = true
		r.Entries[noVoter] = Entry{Value: true, Late: true}
	})
	expectVote(t, "late vote", late, noVoter, VoteLate, true, noVoter)

	expired := poll(10, 4, 4, func(r *Record) { r.Expired = true })
	expectVote(t, "expired without a majority, voted", expired, yesVoter, VoteUndecided, true, yesVoter)
	expectVote(t, "expired without a majority, not voted", expired, silent, VoteNotVoted, true, silent)

	// "failed" with a yes count majority: the flag and the count disagree, so
	// nobody is in a minority.
	failedYes := poll(10, 6, 2, func(r *Record) { r.Failed = true })
	expectVote(t, "failed flag against the count, yes voter", failedYes, yesVoter, VoteUndecided, true, yesVoter)
	expectVote(t, "failed flag against the count, no voter", failedYes, noVoter, VoteUndecided, true, noVoter)

	// Exactly half is not a majority.
	half := poll(10, 5, 2, func(r *Record) { r.Success = true })
	expectVote(t, "half of the participants", half, noVoter, VoteUndecided, true, noVoter)

	// No participant list: a vote can be compared, absence cannot be seen.
	noList := poll(10, 6, 2, func(r *Record) { r.Success = true; r.Participants = nil })
	expectVote(t, "no participant list, minority", noList, noVoter, VoteMinority, true, noVoter)
	expectVote(t, "no participant list, no vote", noList, silent, "", false, silent)
}

func TestSigning(t *testing.T) {
	r := load(t, "vm_proofs.json", SignValue)[0]
	if len(r.Participants) != 26 || !r.Success {
		t.Fatalf("unexpected record: %d participants, success=%v", len(r.Participants), r.Success)
	}
	check := func(name, signer string, want SigningResult, wantCounted bool) {
		t.Helper()
		got, counted := ClassifySigning(r, signer)
		if got != want || counted != wantCounted {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", name, got, counted, want, wantCounted)
		}
	}
	check("our verifier", ourVerifier, SigningSigned, true)
	check("participant without a signature", absentVerifier, SigningNotSigned, true)
	check("account outside the session", stranger, "", false)

	r.Success = false
	check("open session", absentVerifier, "", false)
}

func TestParseSkipsRecordsWithoutTime(t *testing.T) {
	records, err := ParseRecords([]byte(`{"data":[{"id":"1","success":true},{"id":"2","created_at":{"ms":1795543506000},"failed":true,"axelar1a":{"vote":false},"axelarvaloper1b":{"vote":true},"axelar1c":"text"}]}`), VoteValue)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ID != "2" || len(records[0].Entries) != 1 {
		t.Fatalf("unexpected records: %+v", records)
	}
}
