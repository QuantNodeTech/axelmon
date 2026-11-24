package api

import (
	"bharvest.io/axelmon/log"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/axelarnetwork/axelar-core/x/nexus/exported"
	"io"
	"net/http"
	"time"
)

type PollingType string

const (
	EVM_POLLING_TYPE PollingType = "searchPolls"
	VM_POLLING_TYPE  PollingType = "searchVMPolls"
)

func (c *Client) GetVerifierSupportedChains(proxyAcc string) ([]exported.ChainName, error) {
	url := fmt.Sprintf("%s/validator/getVerifiers", c.axelarscan)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New("Failed to get proper data from axelarscan")
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res map[string]any
	err = json.Unmarshal(bodyBytes, &res)
	if err != nil {
		return nil, err
	}

	dataBytes, err := json.Marshal(res["data"])
	if err != nil {
		return nil, err
	}

	var data []VerifierAccount
	err = json.Unmarshal(dataBytes, &data)
	if err != nil {
		return nil, err
	}

	for _, d := range data {
		if d.Address == proxyAcc {
			var result []exported.ChainName
			for _, chainName := range d.SupportedChains {
				result = append(result, exported.ChainName(chainName))
			}
			return result, nil
		}
	}
	return nil, errors.New("didn't found any verifier matched with your Acc -> " + proxyAcc)
}

// Search methods of the axelarscan validator API.
const (
	VM_SIGNING_TYPE PollingType = "searchVMProofs"
)

// GetRecords returns the polls or signing sessions of a chain that were
// created since `since`, newest first, at most `size` of them.
func (c *Client) GetRecords(method PollingType, chain string, size int, since time.Time, valueKey ValueKey) ([]Record, error) {
	if size > 255 {
		return nil, errors.New("exceed maximum # of records (max 255)")
	}

	reqBytes, err := json.Marshal(RecordsRequest{
		Method:   string(method),
		Chain:    chain,
		Size:     size,
		FromTime: since.Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/validator/%s", c.axelarscan, method)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response from axelarscan: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	records, err := ParseRecords(bodyBytes, valueKey)
	if err != nil {
		return nil, err
	}

	// The API filters by time already. Filter again, so that a server that
	// ignores fromTime cannot widen the window.
	inWindow := records[:0]
	for _, r := range records {
		if r.Created.Before(since) {
			log.Debug("skipping old record")
			continue
		}
		inWindow = append(inWindow, r)
	}
	return inWindow, nil
}
