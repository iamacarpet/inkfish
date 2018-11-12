package inkfish

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"github.com/pkg/errors"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
)

const PasswordHashLen = 64 // Length of SHA-256 output, as hex chars

type Acl struct {
	From    []string
	Entries []AclEntry
}

type AclEntry struct {
	AllMethods bool
	Methods    []string
	Pattern    *regexp.Regexp
}

type UserEntry struct {
	Username     string
	PasswordHash string
}

func listContainsString(haystack []string, needle string) bool {
	// Return true iff needle is present in haystack
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func (c *Inkfish) Permits(from, method, url string) bool {
	// Check each acl in the config to see if it permits the request
	for _, aclConfig := range c.Acls {
		if aclConfig.Permits(from, method, url) {
			return true
		}
	}
	return false
}

func (c *Inkfish) CredentialsAreValid(user, password string) bool {
	// Check each UserEntry to see if it matches the provided credentials
	hashedPw := sha256.Sum256([]byte(password))
	for _, ent := range c.Passwd {
		if ent.Username == user {
			// It's possible to have multiple passwords for the same user,
			// this allows blue/green credentials. Otherwise we could early-exit.
			actualPw, err := hex.DecodeString(ent.PasswordHash)
			if err != nil {
				return false
			}
			if subtle.ConstantTimeCompare(hashedPw[:], actualPw) == 1 {
				return true
			}
		}
	}
	return false
}

func (c *Acl) Permits(from, method, url string) bool {
	// Check whether an acl permits a request. 2 things must be true:
	// 1) The requesting user must be present in the "From" list of the Acl
	// 2) The request method and url must match one of the Acl entries
	if !listContainsString(c.From, from) {
		return false
	}
	for _, e := range c.Entries {
		if e.AllMethods || listContainsString(e.Methods, method) {
			if e.Pattern.MatchString(url) {
				return true
			}
		}
	}
	return false
}

func parseAclEntry(words []string) (*AclEntry, error) {
	// Take a config line like ["url", "GET", ""] and turn it into an AclEntry
	var aclUrl AclEntry
	if len(words) != 2 && len(words) != 3 {
		return nil, errors.New("wrong number of fields (expecting 2 or 3)")
	}
	if words[0] != "url" {
		return nil, errors.New("expecting entry to start with 'url'")
	}
	var urlPattern string
	if len(words) == 2 {
		// acl <regexp>
		aclUrl.AllMethods = true
		urlPattern = words[1]
	} else { // == 3
		// acl <methodlist> <regexp>
		aclUrl.AllMethods = false
		aclUrl.Methods = strings.Split(words[1], ",")
		urlPattern = words[2]
	}
	re, err := regexp.Compile(urlPattern)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse url pattern")
	}
	aclUrl.Pattern = re
	return &aclUrl, nil
}

func parseAcl(lines []string) (*Acl, error) {
	// Take a list of config lines and turn it into an Acl
	var aclConfig Acl
	for line_no, l := range lines {
		l = strings.TrimLeft(l, " \t")
		if len(l) == 0 || l[0] == '#' {
			continue
		}
		words := strings.Fields(l)
		if words[0] == "from" {
			aclConfig.From = append(aclConfig.From, words[1:]...)
		} else if words[0] == "url" {
			newEntry, err := parseAclEntry(words)
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("config error at line: %v", line_no+1))
			}
			aclConfig.Entries = append(aclConfig.Entries, *newEntry)
		} else {
			return nil, errors.Errorf("unknown directive at line: %v", line_no+1)
		}
	}
	return &aclConfig, nil
}

func loadAclFromFile(data []byte) (*Acl, error) {
	lines := strings.Split(string(data), "\n")
	result, err := parseAcl(lines)
	if err != nil {
		return nil, errors.Wrapf(err, "error loading acls")
	}
	return result, nil
}

func loadUsersFromFile(data []byte) ([]UserEntry, error) {
	var result []UserEntry
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.Trim(line, " \t")
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		fields := strings.Split(line, ":")
		username, passwordHash := fields[0], fields[1]
		if len(username) == 0 || len(passwordHash) != PasswordHashLen {
			// TODO: logging
			continue
		}
		result = append(result, UserEntry{
			Username: username,
			PasswordHash: passwordHash,
		})
	}
	return result, nil
}

func (proxy *Inkfish) LoadConfigFromDirectory(configDir string) (error) {
	// Load ACLs and passwd entries from a directory
	files, err := ioutil.ReadDir(configDir)
	if err != nil {
		msg := fmt.Sprintf("failed to list config dir: %v", configDir)
		return errors.Wrap(err, msg)
	}
	for _, fi := range files {
		if fi.IsDir() {
			continue
		}
		fullpath := filepath.Join(configDir, fi.Name())
		data, err := ioutil.ReadFile(fullpath)
		if err != nil {
			return errors.Wrapf(err, "failed read config file: %v", fullpath)
		}
		if filepath.Ext(fi.Name()) == ".conf" {
			acl, err := loadAclFromFile(data)
			if err != nil {
				return errors.Wrapf(err, "error in acl file: %v", fullpath)
			}
			proxy.Acls = append(proxy.Acls, *acl)
		} else if filepath.Ext(fi.Name()) == ".passwd" {
			userRecords, err := loadUsersFromFile(data)
			if err != nil {
				return errors.Wrapf(err, "error in passwd file: %v", fullpath)
			}
			proxy.Passwd = append(proxy.Passwd, userRecords...)
		}
	}
	return nil
}
