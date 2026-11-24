package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bharvest.io/axelmon/client/api"
	"bharvest.io/axelmon/log"
	"bharvest.io/axelmon/metrics"
	"bharvest.io/axelmon/server"
)

// checkVMSignings checks that the verifier signed in the multisig signing
// sessions of its chains. A session where the verifier was a participant and
// sent no signature counts as not signed.
func (c *Config) checkVMSignings(ctx context.Context) error {
	signer := c.Wallet.Proxy.PrintAcc()
	chains, err := api.C.GetVerifierSupportedChains(signer)
	if err != nil {
		return err
	}

	result := make(map[string]server.VotesInfo)
	since := time.Now().Add(-c.checkWindow())
	size := c.checkSize()

	for _, chain := range chains {
		if c.General.ExceptChains[strings.ToLower(chain.String())] {
			continue
		}

		records, err := api.C.GetRecords(api.VM_SIGNING_TYPE, chain.String(), size, since, api.SignValue)
		if err != nil {
			return fmt.Errorf("error polling signings for chain %s: %w", chain, err)
		}
		if len(records) >= size {
			log.Warn(fmt.Sprintf("%s %s: %d records fill the request size; older sessions of the check window are not checked. Raise check_n or lower the check period.", api.VM_SIGNING_TYPE, chain, len(records)))
		}

		counts := make(map[api.SigningResult]int)
		for _, r := range records {
			if res, counted := api.ClassifySigning(r, signer); counted {
				counts[res]++
			}
		}
		total := 0
		for _, r := range api.SigningResults {
			metrics.SetSignings(c.General.Network, chain.String(), signer, string(r), counts[r])
			total += counts[r]
		}
		missed := counts[api.SigningNotSigned]
		info := server.VotesInfo{
			Missed:    fmt.Sprintf("%d / %d", missed, total),
			MissedCnt: missed,
			TotalCnt:  total,
			Status:    true,
		}

		name := fmt.Sprintf("%s status(%s:%s)", api.VM_SIGNING_TYPE, chain, signer)
		if total > 0 && float64(missed)/float64(total)*100 > float64(c.PollingVote.MissPercentage) {
			info.Status = false
			c.alert(name, []string{fmt.Sprintf("not signed %d of %d sessions", missed, total)}, false, false)
		} else {
			c.alert(name, []string{}, true, false)
		}

		result[chain.String()+"|"+signer] = info
	}

	server.GlobalState.VMSignings.Chain = result
	metrics.SetLastCheck(c.General.Network, "vm_signing")
	return nil
}
