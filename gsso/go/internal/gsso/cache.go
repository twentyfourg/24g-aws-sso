package gsso

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type portalCachePair struct {
	AccountID   string `json:"accountId"`
	AccountName string `json:"accountName"`
	RoleName    string `json:"roleName"`
}

type portalCacheFile struct {
	StartURL  string            `json:"startUrl"`
	FetchedAt string            `json:"fetchedAt"`
	Pairs     []portalCachePair `json:"pairs"`
}

// ReadPortalCache returns the cached account/role matrix, or ok=false if the
// cache is missing or unusable. It stores account and role names only, never
// a token.
func ReadPortalCache(paths Paths) (fetchedAt string, pairs []Pair, ok bool) {
	data, err := os.ReadFile(paths.PortalCachePath)
	if err != nil {
		return "", nil, false
	}
	var file portalCacheFile
	if err := json.Unmarshal(data, &file); err != nil {
		return "", nil, false
	}
	if file.StartURL != SSOStartURL {
		return "", nil, false
	}
	for _, p := range file.Pairs {
		if p.AccountID == "" || p.AccountName == "" || p.RoleName == "" {
			return "", nil, false
		}
		pairs = append(pairs, Pair{AccountID: p.AccountID, AccountName: p.AccountName, RoleName: p.RoleName})
	}
	fetchedAt = file.FetchedAt
	if fetchedAt == "" {
		fetchedAt = "unknown"
	}
	return fetchedAt, pairs, true
}

// WritePortalCache overwrites the portal cache with pairs, sorted for a
// stable diff.
func WritePortalCache(paths Paths, pairs []Pair) error {
	sorted := append([]Pair(nil), pairs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].AccountID != sorted[j].AccountID {
			return sorted[i].AccountID < sorted[j].AccountID
		}
		if sorted[i].AccountName != sorted[j].AccountName {
			return sorted[i].AccountName < sorted[j].AccountName
		}
		return sorted[i].RoleName < sorted[j].RoleName
	})
	file := portalCacheFile{
		StartURL:  SSOStartURL,
		FetchedAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	for _, p := range sorted {
		file.Pairs = append(file.Pairs, portalCachePair{AccountID: p.AccountID, AccountName: p.AccountName, RoleName: p.RoleName})
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.PortalCachePath), 0o755); err != nil {
		return err
	}
	return AtomicWrite(paths.PortalCachePath, string(data)+"\n")
}

// LoadRemotePairs returns the portal account/role matrix: the cache by
// default, or a live Identity Center query on a cache miss or hardRefresh.
func LoadRemotePairs(paths Paths, hardRefresh bool) ([]Pair, error) {
	if !hardRefresh {
		if fetchedAt, pairs, ok := ReadPortalCache(paths); ok {
			plural := "s"
			if len(pairs) == 1 {
				plural = ""
			}
			Err("Using portal cache from %s (%d role%s). Refresh with --hard-refresh.", fetchedAt, len(pairs), plural)
			return pairs, nil
		}
	}
	token, err := LoadAccessToken(paths)
	if err != nil {
		return nil, err
	}
	pairs, err := FetchRemotePairs(token)
	if err != nil {
		return nil, err
	}
	if err := WritePortalCache(paths, pairs); err != nil {
		return nil, err
	}
	Err("Wrote %s", paths.PortalCachePath)
	return pairs, nil
}
