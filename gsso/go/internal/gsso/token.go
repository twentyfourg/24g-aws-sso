package gsso

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// timestampRE matches the three expiresAt shapes the AWS CLI has written
// over time: "...Z", "...UTC", and "...+05:00" / "...+0500", with an
// optional fractional-second component and a space or "T" separator.
var timestampRE = regexp.MustCompile(
	`^(\d{4})-(\d{2})-(\d{2})[T ](\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?\s*(Z|UTC|[+-]\d{2}:?\d{2})?$`,
)

// ParseTimestamp parses an SSO cache expiresAt value, or returns false if it
// does not match any known shape.
func ParseTimestamp(raw string) (time.Time, bool) {
	m := timestampRE.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return time.Time{}, false
	}
	year, _ := strconv.Atoi(m[1])
	month, _ := strconv.Atoi(m[2])
	day, _ := strconv.Atoi(m[3])
	hour, _ := strconv.Atoi(m[4])
	minute, _ := strconv.Atoi(m[5])
	second, _ := strconv.Atoi(m[6])
	fraction := m[7]
	offset := m[8]

	nanos := 0
	if fraction != "" {
		padded := (fraction + "000000")[:6]
		micro, _ := strconv.Atoi(padded)
		nanos = micro * 1000
	}

	loc := time.UTC
	if offset != "" && offset != "Z" && offset != "UTC" {
		normalized := strings.ReplaceAll(offset, ":", "")
		sign := 1
		if normalized[0] == '-' {
			sign = -1
		}
		oh, _ := strconv.Atoi(normalized[1:3])
		om, _ := strconv.Atoi(normalized[3:5])
		secondsEast := sign * (oh*3600 + om*60)
		loc = time.FixedZone(offset, secondsEast)
	}
	return time.Date(year, time.Month(month), day, hour, minute, second, nanos, loc), true
}

type ssoCacheEntry struct {
	AccessToken string `json:"accessToken"`
	StartURL    string `json:"startUrl"`
	ExpiresAt   string `json:"expiresAt"`
}

// LoadAccessToken returns the newest unexpired 24G access token from the AWS
// CLI's SSO cache. It never logs or returns the token except to the caller.
func LoadAccessToken(paths Paths) (string, error) {
	entries, err := os.ReadDir(paths.SSOCacheDir)
	if err != nil {
		return "", NewErrorWithFix(ExAuth, "run `gsso login`", "no SSO cache directory")
	}
	now := time.Now().UTC()
	var bestExpires time.Time
	var bestToken string
	found := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(paths.SSOCacheDir, entry.Name()))
		if err != nil {
			continue
		}
		var cache ssoCacheEntry
		if err := json.Unmarshal(data, &cache); err != nil {
			continue // client registration files and partial writes
		}
		if cache.AccessToken == "" || cache.StartURL != SSOStartURL {
			continue
		}
		expires, ok := ParseTimestamp(cache.ExpiresAt)
		if !ok || !expires.After(now) {
			continue
		}
		if !found || expires.After(bestExpires) {
			bestExpires = expires
			bestToken = cache.AccessToken
			found = true
		}
	}
	if !found {
		return "", NewErrorWithFix(ExAuth, "run `gsso login`", "no valid 24G SSO session")
	}
	return bestToken, nil
}
