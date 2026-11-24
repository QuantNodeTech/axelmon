package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bharvest.io/axelmon/client/api"
	"bharvest.io/axelmon/client/grpc"
	"bharvest.io/axelmon/log"
	"bharvest.io/axelmon/metrics"
	"bharvest.io/axelmon/server"
	"github.com/axelarnetwork/axelar-core/x/nexus/exported"
)

const defaultCheckN = 100

func (c *Config) checkEVMVotes(ctx context.Context) error {
	client := grpc.New(c.General.GRPC)
	err := client.Connect(ctx, c.General.GRPCSecureConnection)
	defer client.Terminate(ctx)
	if err != nil {
		return err
	}

	chains, err := client.GetChains(ctx)
	if err != nil {
		return err
	}

	// A validator votes with its proxy account. The participant list of a
	// validator poll names the validator by its axelarvaloper address.
	return c.checkPollingVotes(ctx, api.EVM_POLLING_TYPE, chains, c.Wallet.Validator.PrintValoper())
}

func (c *Config) checkVMVotes(ctx context.Context) error {
	chains, err := api.C.GetVerifierSupportedChains(c.Wallet.Proxy.PrintAcc())
	if err != nil {
		return err
	}
	// A verifier votes with its own account, and the participant list names it.
	return c.checkPollingVotes(ctx, api.VM_POLLING_TYPE, chains, c.Wallet.Proxy.PrintAcc())
}

// checkWindow is how far back the vote and signing checks look.
func (c *Config) checkWindow() time.Duration {
	if c.PollingVote.CheckPeriodHours > 0 {
		return time.Duration(c.PollingVote.CheckPeriodHours) * time.Hour
	}
	if c.PollingVote.CheckPeriodDays == 0 {
		c.PollingVote.CheckPeriodDays = 10
	}
	return time.Duration(c.PollingVote.CheckPeriodDays) * 24 * time.Hour
}

func (c *Config) checkSize() int {
	if c.PollingVote.CheckN <= 0 {
		return defaultCheckN
	}
	return c.PollingVote.CheckN
}

// legacyVotes rebuilds the counts behind vm_votes_total and evm_votes_total:
// per voter, every vote that is late or not "yes" counts as missed. It does
// not look at the outcome of the poll. Use axelmon_poll_votes for alerts.
func legacyVotes(records []api.Record) map[string]*server.VotesInfo {
	perVoter := make(map[string]*server.VotesInfo)
	for _, r := range records {
		for voter, entry := range r.Entries {
			v := perVoter[voter]
			if v == nil {
				v = &server.VotesInfo{}
				perVoter[voter] = v
			}
			v.TotalCnt++
			if entry.Late || !entry.Value {
				v.MissedCnt++
			}
		}
	}
	return perVoter
}

// countVotes classifies every record for one account.
func countVotes(records []api.Record, voter string, participantIDs ...string) map[api.VoteResult]int {
	counts := make(map[api.VoteResult]int)
	for _, r := range records {
		if result, counted := api.ClassifyVote(r, voter, participantIDs...); counted {
			counts[result]++
		}
	}
	return counts
}

func (c *Config) checkPollingVotes(ctx context.Context, pollingType api.PollingType, chains []exported.ChainName, participantID string) error {
	result := make(map[string]server.VotesInfo)
	voter := c.Wallet.Proxy.PrintAcc()
	since := time.Now().Add(-c.checkWindow())
	size := c.checkSize()

	kind := "evm"
	if pollingType == api.VM_POLLING_TYPE {
		kind = "vm"
	}

	for _, chain := range chains {
		// Skip excepted chains
		if c.General.ExceptChains[strings.ToLower(chain.String())] {
			continue
		}

		records, err := api.C.GetRecords(pollingType, chain.String(), size, since, api.VoteValue)
		if err != nil {
			return fmt.Errorf("error polling votes for chain %s: %w", chain, err)
		}
		if len(records) >= size {
			log.Warn(fmt.Sprintf("%s %s: %d records fill the request size; older polls of the check window are not checked. Raise check_n or lower the check period.", pollingType, chain, len(records)))
		}

		// Legacy metrics, per voter and per chain. The label order is historic:
		// "chain" holds the network and "network" holds the chain.
		for validator, info := range legacyVotes(records) {
			if pollingType == api.VM_POLLING_TYPE {
				metrics.SetVMVotesMissed(c.General.Network, validator, chain.String(), info.MissedCnt)
				metrics.SetVMVotesSuccess(c.General.Network, validator, chain.String(), info.TotalCnt-info.MissedCnt)
			} else {
				metrics.SetEVMVotesMissed(c.General.Network, validator, chain.String(), info.MissedCnt)
				metrics.SetEVMVotesSuccess(c.General.Network, validator, chain.String(), info.TotalCnt-info.MissedCnt)
			}
		}

		// The monitored account: compare each vote with the outcome of its poll.
		counts := countVotes(records, voter, participantID)
		total := 0
		for _, r := range api.VoteResults {
			metrics.SetPollVotes(c.General.Network, kind, chain.String(), voter, string(r), counts[r])
			total += counts[r]
		}
		bad := counts[api.VoteMinority] + counts[api.VoteNotVoted] + counts[api.VoteLate]
		info := server.VotesInfo{
			Missed:    fmt.Sprintf("%d / %d", bad, total),
			MissedCnt: bad,
			TotalCnt:  total,
			Status:    true,
		}

		// A "no" vote with the majority is not a miss. Only a vote against the
		// majority, a missing vote and a late vote count.
		name := fmt.Sprintf("%s status(%s:%s)", pollingType, chain, voter)
		if total > 0 && float64(bad)/float64(total)*100 > float64(c.PollingVote.MissPercentage) {
			info.Status = false
			c.alert(name, []string{fmt.Sprintf("minority %d, not voted %d, late %d of %d polls",
				counts[api.VoteMinority], counts[api.VoteNotVoted], counts[api.VoteLate], total)}, false, false)
		} else {
			c.alert(name, []string{}, true, false)
		}

		result[chain.String()+"|"+voter] = info
	}

	server.GlobalState.EVMVotes.Chain = result
	metrics.SetLastCheck(c.General.Network, kind)
	return nil
}
