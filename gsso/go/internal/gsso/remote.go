package gsso

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"
)

// Pair is one (account, role) grant the Identity Center portal reports.
type Pair struct {
	AccountID   string
	AccountName string
	RoleName    string
}

type accountListResponse struct {
	AccountList []struct {
		AccountID   string `json:"accountId"`
		AccountName string `json:"accountName"`
	} `json:"accountList"`
}

type roleListResponse struct {
	RoleList []struct {
		RoleName string `json:"roleName"`
	} `json:"roleList"`
}

// fetchAccountRoles lists roles in one account, retrying on the Identity
// Center read throttle (TooManyRequestsException).
func fetchAccountRoles(token, accountID string) ([]string, error) {
	delay := 500 * time.Millisecond
	var lastErr error
	for i := 0; i < RoleFetchRetries; i++ {
		out, err := RunAWS([]string{
			"sso", "list-account-roles",
			"--access-token", token,
			"--region", SSORegion,
			"--account-id", accountID,
		}, token)
		if err == nil {
			var resp roleListResponse
			if jsonErr := json.Unmarshal([]byte(out), &resp); jsonErr != nil {
				return nil, jsonErr
			}
			roles := make([]string, len(resp.RoleList))
			for i, r := range resp.RoleList {
				roles[i] = r.RoleName
			}
			return roles, nil
		}
		lastErr = err
		if !strings.Contains(err.Error(), "TooManyRequests") {
			return nil, err
		}
		time.Sleep(delay)
		delay *= 2
	}
	return nil, lastErr
}

// FetchRemotePairs returns every (account, role) pair the signed-in user has
// in Identity Center, listing roles across accounts concurrently.
func FetchRemotePairs(token string) ([]Pair, error) {
	out, err := RunAWS([]string{
		"sso", "list-accounts",
		"--access-token", token,
		"--region", SSORegion,
	}, token)
	if err != nil {
		return nil, err
	}
	var resp accountListResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, err
	}
	if len(resp.AccountList) == 0 {
		return nil, nil
	}
	workers := RoleFetchConcurrency
	if len(resp.AccountList) < workers {
		workers = len(resp.AccountList)
	}
	Err("Listing roles in %d accounts (%d at a time)", len(resp.AccountList), workers)

	type job = struct {
		AccountID   string
		AccountName string
	}
	jobs := make(chan job)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var pairs []Pair
	var firstErr error

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				roles, err := fetchAccountRoles(token, j.AccountID)
				mu.Lock()
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
				} else {
					for _, role := range roles {
						pairs = append(pairs, Pair{AccountID: j.AccountID, AccountName: j.AccountName, RoleName: role})
					}
				}
				mu.Unlock()
			}
		}()
	}
	for _, a := range resp.AccountList {
		name := a.AccountName
		if name == "" {
			name = a.AccountID
		}
		jobs <- job{AccountID: a.AccountID, AccountName: name}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].AccountID != pairs[j].AccountID {
			return pairs[i].AccountID < pairs[j].AccountID
		}
		if pairs[i].AccountName != pairs[j].AccountName {
			return pairs[i].AccountName < pairs[j].AccountName
		}
		return pairs[i].RoleName < pairs[j].RoleName
	})
	return pairs, nil
}
