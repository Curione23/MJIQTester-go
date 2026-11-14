package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
)

var snowflakeRe = regexp.MustCompile(`^\d{17,20}$`)

type Banlist struct {
	mu    sync.Mutex
	path  string
	banned map[string]struct{}
}

func NewBanlist(path string) *Banlist {
	return &Banlist{path: path, banned: map[string]struct{}{}}
}

func (b *Banlist) Load() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.banned = map[string]struct{}{}
	f, err := os.Open(b.path)
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.WriteFile(b.path, []byte{}, 0644)
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	count := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if snowflakeRe.MatchString(line) {
			b.banned[line] = struct{}{}
			count++
		} else {
			fmt.Fprintf(os.Stderr, "Ignoring malformed banlist entry: %q\n", line)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	fmt.Printf("Loaded %d verification ban entries.\n", count)
	return nil
}

func (b *Banlist) IsBanned(userID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.banned[userID]
	return ok
}

func (b *Banlist) Ban(userID string) (bool, error) {
	if !snowflakeRe.MatchString(userID) {
		return false, fmt.Errorf("invalid user ID")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.banned[userID]; ok {
		return false, nil
	}
	f, err := os.OpenFile(b.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	if _, err := f.WriteString(userID + "\n"); err != nil {
		f.Close()
		return false, err
	}
	if err := f.Close(); err != nil {
		return false, err
	}
	b.banned[userID] = struct{}{}
	return true, nil
}

func (b *Banlist) Unban(userID string) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.banned[userID]; !ok {
		return false, nil
	}
	content, err := os.ReadFile(b.path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	var remaining []string
	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || line == userID {
			continue
		}
		remaining = append(remaining, line)
	}
	out := ""
	if len(remaining) > 0 {
		out = strings.Join(remaining, "\n") + "\n"
	}
	if err := os.WriteFile(b.path, []byte(out), 0644); err != nil {
		return false, err
	}
	delete(b.banned, userID)
	return true, nil
}
