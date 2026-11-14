package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Token                    string
	ClientID                 string
	VerifiedRoleID           string
	VerdictChannelID         string
	VerificationTimeoutMins  int
	VerificationBanlistPath  string
}

func loadConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to load %s: %w", path, err)
	}
	defer f.Close()

	values := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.Index(line, "=")
		if i == -1 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		if key == "" {
			continue
		}
		values[key] = strings.TrimSpace(line[i+1:])
	}
	if err := sc.Err(); err != nil {
		return Config{}, err
	}

	baseDir := filepath.Dir(path)
	cfg := Config{
		Token:                   values["TOKEN"],
		ClientID:                values["CLIENT_ID"],
		VerifiedRoleID:          values["VERIFIED_ROLE_ID"],
		VerdictChannelID:        values["VERDICT_CHANNEL_ID"],
		VerificationTimeoutMins: 3,
	}
	if v := values["VERIFICATION_TIMEOUT_MINUTES"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.VerificationTimeoutMins = n
		}
	}
	if p := values["VERIFICATION_BANLIST_PATH"]; p != "" {
		if filepath.IsAbs(p) {
			cfg.VerificationBanlistPath = p
		} else {
			cfg.VerificationBanlistPath = filepath.Join(baseDir, p)
		}
	} else {
		cfg.VerificationBanlistPath = filepath.Join(baseDir, "verification-bans.txt")
	}

	var missing []string
	for _, k := range []string{"TOKEN", "CLIENT_ID", "VERIFIED_ROLE_ID", "VERDICT_CHANNEL_ID"} {
		if values[k] == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required config values in config.cfg: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}
