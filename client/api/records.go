package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// A Record is one poll (searchPolls, searchVMPolls) or one signing session
// (searchVMProofs) as axelarscan returns it.
//
// axelarscan stores one object per poll. Its meta fields say how the poll
// ended and who had to take part. Every account that voted or signed has its
// own field, with the account address as the key. An account that did nothing
// has no field at all, so absence is only visible through Participants.
type Record struct {
	ID           string
	Created      time.Time
	Success      bool
	Failed       bool
	Expired      bool
	Participants []string
	// Entries holds the vote or the signature of each account that sent one.
	Entries map[string]Entry
}

type Entry struct {
	// Value is the "vote" field of a poll or the "sign" field of a signing session.
	Value bool
	// Late is set by axelarscan when a vote arrived after the poll completed.
	Late bool
}

// Finished is false while the poll or the session is still open.
func (r Record) Finished() bool {
	return r.Success || r.Failed || r.Expired
}

// IsParticipant reports whether one of ids is in the participant list.
// known is false when the record has no participant list.
func (r Record) IsParticipant(ids ...string) (participant, known bool) {
	if len(r.Participants) == 0 {
		return false, false
	}
	for _, p := range r.Participants {
		for _, id := range ids {
			if id != "" && p == id {
				return true, true
			}
		}
	}
	return false, true
}

// ValueKey names the field of an entry that holds its vote or its signature.
type ValueKey string

const (
	VoteValue ValueKey = "vote"
	SignValue ValueKey = "sign"
)

// ParseRecords decodes an axelarscan search response.
func ParseRecords(body []byte, valueKey ValueKey) ([]Record, error) {
	var res struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	records := make([]Record, 0, len(res.Data))
	for _, raw := range res.Data {
		var created struct {
			Ms float64 `json:"ms"`
		}
		if json.Unmarshal(raw["created_at"], &created) != nil || created.Ms == 0 {
			// A record without a time cannot be placed in the check window.
			continue
		}
		r := Record{
			ID:      rawString(raw["id"]),
			Created: time.UnixMilli(int64(created.Ms)),
			Entries: make(map[string]Entry),
		}
		_ = json.Unmarshal(raw["success"], &r.Success)
		_ = json.Unmarshal(raw["failed"], &r.Failed)
		_ = json.Unmarshal(raw["expired"], &r.Expired)
		_ = json.Unmarshal(raw["participants"], &r.Participants)

		for key, value := range raw {
			// Only account fields hold a vote or a signature. Validator polls list
			// their participants as axelarvaloper addresses, so the "1" matters.
			if !strings.HasPrefix(key, "axelar1") {
				continue
			}
			var fields map[string]json.RawMessage
			if json.Unmarshal(value, &fields) != nil {
				continue
			}
			var entry Entry
			if json.Unmarshal(fields[string(valueKey)], &entry.Value) != nil {
				continue
			}
			_ = json.Unmarshal(fields["late"], &entry.Late)
			r.Entries[key] = entry
		}
		records = append(records, r)
	}
	return records, nil
}

func rawString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	// Validator poll ids are strings, but be tolerant of a bare number.
	return strings.Trim(string(raw), "\"")
}

// VoteResult is how one account behaved in one poll.
type VoteResult string

const (
	// VoteAgreed: the account voted as the majority did. A "no" vote with a
	// "no" majority is agreement, not a miss.
	VoteAgreed VoteResult = "agreed"
	// VoteMinority: the account voted against the majority.
	VoteMinority VoteResult = "minority"
	// VoteNotVoted: the poll ended and the account, a participant, sent no vote.
	VoteNotVoted VoteResult = "not_voted"
	// VoteLate: the vote arrived after the poll completed.
	VoteLate VoteResult = "late"
	// VoteUndecided: the account voted, but the poll has no clear majority to
	// compare with.
	VoteUndecided VoteResult = "undecided"
)

var VoteResults = []VoteResult{VoteAgreed, VoteMinority, VoteNotVoted, VoteLate, VoteUndecided}

// Majority returns the side that won the poll. decided is false when the poll
// has no clear winner.
//
// Both signals must agree: the outcome flag of axelarscan and the count of the
// votes. A side wins the count when more than half of all participants voted
// for it. Validator votes are weighted by stake, so the count alone is not
// proof, and the "failed" flag alone does not say that "no" won.
func (r Record) Majority() (yes bool, decided bool) {
	var yesCnt, noCnt int
	for _, e := range r.Entries {
		if e.Late {
			continue
		}
		if e.Value {
			yesCnt++
		} else {
			noCnt++
		}
	}
	total := len(r.Participants)
	if total == 0 {
		total = yesCnt + noCnt
	}
	switch {
	case r.Success && yesCnt*2 > total:
		return true, true
	case r.Failed && noCnt*2 > total:
		return false, true
	}
	return false, false
}

// ClassifyVote says how the account behaved in the poll. voter is the account
// that sends the votes. participantIDs are the names the account can have in
// the participant list: the same account for a verifier, the axelarvaloper
// address for a validator.
//
// counted is false when the poll says nothing about the account: the poll is
// still open, the account was not a participant, or the record cannot tell.
func ClassifyVote(r Record, voter string, participantIDs ...string) (result VoteResult, counted bool) {
	if !r.Finished() {
		return "", false
	}
	entry, voted := r.Entries[voter]
	if !voted {
		if participant, known := r.IsParticipant(participantIDs...); known && participant {
			return VoteNotVoted, true
		}
		return "", false
	}
	if entry.Late {
		return VoteLate, true
	}
	yes, decided := r.Majority()
	if !decided {
		return VoteUndecided, true
	}
	if entry.Value == yes {
		return VoteAgreed, true
	}
	return VoteMinority, true
}

// SigningResult is how one account behaved in one signing session.
type SigningResult string

const (
	SigningSigned    SigningResult = "signed"
	SigningNotSigned SigningResult = "not_signed"
)

var SigningResults = []SigningResult{SigningSigned, SigningNotSigned}

// ClassifySigning says whether the account signed in the session. counted is
// false while the session is open, and when the account was not a participant.
func ClassifySigning(r Record, signer string) (result SigningResult, counted bool) {
	if !r.Finished() {
		return "", false
	}
	if entry, ok := r.Entries[signer]; ok && entry.Value {
		return SigningSigned, true
	}
	if participant, known := r.IsParticipant(signer); known && participant {
		return SigningNotSigned, true
	}
	return "", false
}
