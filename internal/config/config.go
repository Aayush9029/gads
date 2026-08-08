package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultAPIEndpoint = "https://googleads.googleapis.com"
	DefaultAPIVersion  = "v25"
)

type Profile struct {
	CustomerID       string `json:"customer_id,omitempty"`
	LoginCustomerID  string `json:"login_customer_id,omitempty"`
	LinkedCustomerID string `json:"linked_customer_id,omitempty"`
	Output           string `json:"output,omitempty"`
	APIVersion       string `json:"api_version,omitempty"`
}

type Credentials struct {
	ClientID       string `json:"client_id,omitempty"`
	ClientSecret   string `json:"client_secret,omitempty"`
	DeveloperToken string `json:"developer_token,omitempty"`
	RefreshToken   string `json:"refresh_token,omitempty"`
}

type profilesFile struct {
	Current  string             `json:"current,omitempty"`
	Profiles map[string]Profile `json:"profiles"`
}

type credentialsFile struct {
	Profiles map[string]Credentials `json:"profiles"`
}

type Store interface {
	CurrentProfile() (string, error)
	SetCurrentProfile(string) error
	ListProfiles() (map[string]Profile, error)
	LoadProfile(string) (Profile, error)
	SaveProfile(string, Profile) error
	LoadCredentials(string) (Credentials, error)
	SaveCredentials(string, Credentials) error
	DeleteCredentials(string) error
}

type FileStore struct {
	Dir string
}

func DefaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gads"), nil
}

func (s FileStore) profilesPath() string    { return filepath.Join(s.Dir, "profiles.json") }
func (s FileStore) credentialsPath() string { return filepath.Join(s.Dir, "credentials.json") }

func (s FileStore) CurrentProfile() (string, error) {
	f, err := s.readProfiles()
	if err != nil {
		return "", err
	}
	if f.Current == "" {
		return "default", nil
	}
	return f.Current, nil
}

func (s FileStore) SetCurrentProfile(name string) error {
	name, err := ProfileName(name)
	if err != nil {
		return err
	}
	f, err := s.readProfiles()
	if err != nil {
		return err
	}
	if _, ok := f.Profiles[name]; !ok {
		return fmt.Errorf("profile %q does not exist", name)
	}
	f.Current = name
	return s.writeJSON(s.profilesPath(), f)
}

func (s FileStore) ListProfiles() (map[string]Profile, error) {
	f, err := s.readProfiles()
	return f.Profiles, err
}

func (s FileStore) LoadProfile(name string) (Profile, error) {
	f, err := s.readProfiles()
	if err != nil {
		return Profile{}, err
	}
	p, ok := f.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q does not exist", name)
	}
	return p, nil
}

func (s FileStore) SaveProfile(name string, p Profile) error {
	name, err := ProfileName(name)
	if err != nil {
		return err
	}
	if p.CustomerID, err = NormalizeCustomerID(p.CustomerID); err != nil {
		return fmt.Errorf("customer ID: %w", err)
	}
	if p.LoginCustomerID, err = NormalizeCustomerID(p.LoginCustomerID); err != nil {
		return fmt.Errorf("login customer ID: %w", err)
	}
	if p.LinkedCustomerID, err = NormalizeCustomerID(p.LinkedCustomerID); err != nil {
		return fmt.Errorf("linked customer ID: %w", err)
	}
	if p.APIVersion == "" {
		p.APIVersion = DefaultAPIVersion
	}
	f, err := s.readProfiles()
	if err != nil {
		return err
	}
	f.Profiles[name] = p
	if f.Current == "" {
		f.Current = name
	}
	return s.writeJSON(s.profilesPath(), f)
}

func (s FileStore) LoadCredentials(name string) (Credentials, error) {
	f, err := s.readCredentials()
	if err != nil {
		return Credentials{}, err
	}
	c, ok := f.Profiles[name]
	if !ok {
		return Credentials{}, fmt.Errorf("credentials for profile %q do not exist", name)
	}
	return c, nil
}

func (s FileStore) SaveCredentials(name string, c Credentials) error {
	name, err := ProfileName(name)
	if err != nil {
		return err
	}
	f, err := s.readCredentials()
	if err != nil {
		return err
	}
	f.Profiles[name] = c
	return s.writeJSON(s.credentialsPath(), f)
}

func (s FileStore) DeleteCredentials(name string) error {
	f, err := s.readCredentials()
	if err != nil {
		return err
	}
	delete(f.Profiles, name)
	return s.writeJSON(s.credentialsPath(), f)
}

func (s FileStore) readProfiles() (profilesFile, error) {
	f := profilesFile{Profiles: map[string]Profile{}}
	err := s.readJSON(s.profilesPath(), &f)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	return f, err
}

func (s FileStore) readCredentials() (credentialsFile, error) {
	f := credentialsFile{Profiles: map[string]Credentials{}}
	err := s.readJSON(s.credentialsPath(), &f)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if f.Profiles == nil {
		f.Profiles = map[string]Credentials{}
	}
	return f, err
}

func (s FileStore) readJSON(path string, dst any) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("refusing insecure permissions %04o on %s; run chmod 600 %s", info.Mode().Perm(), path, path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func (s FileStore) writeJSON(path string, value any) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(s.Dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(s.Dir, ".gads-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func NormalizeCustomerID(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "-", ""))
	if value == "" {
		return "", nil
	}
	if len(value) != 10 {
		return "", fmt.Errorf("must contain exactly 10 digits")
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("must contain digits only")
		}
	}
	return value, nil
}

func ProfileName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("profile name is empty")
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return "", errors.New("profile names may contain letters, digits, hyphens, and underscores")
		}
	}
	return value, nil
}
