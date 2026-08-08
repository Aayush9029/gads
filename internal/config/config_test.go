package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNormalizeCustomerID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "plain", input: "1234567890", want: "1234567890"},
		{name: "hyphenated", input: "123-456-7890", want: "1234567890"},
		{name: "surrounding whitespace", input: " \t123-456-7890\n", want: "1234567890"},
		{name: "empty", input: "", want: ""},
		{name: "whitespace only", input: "  \n", want: ""},
		{name: "too short", input: "123456789", wantErr: "exactly 10 digits"},
		{name: "too long", input: "12345678901", wantErr: "exactly 10 digits"},
		{name: "letters", input: "12345abcde", wantErr: "digits only"},
		{name: "spaces within", input: "123 456 7890", wantErr: "exactly 10 digits"},
		{name: "unicode digits", input: "١٢٣٤٥٦٧٨٩٠", wantErr: "exactly 10 digits"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeCustomerID(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NormalizeCustomerID(%q) error = %v, want containing %q", tt.input, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeCustomerID(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeCustomerID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProfileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		want    string
		wantErr string
	}{
		{input: "production_1-us", want: "production_1-us"},
		{input: " Work ", want: "Work"},
		{input: "", wantErr: "empty"},
		{input: "   ", wantErr: "empty"},
		{input: "two words", wantErr: "letters, digits"},
		{input: "profile/name", wantErr: "letters, digits"},
		{input: "på", wantErr: "letters, digits"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := ProfileName(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ProfileName(%q) error = %v, want containing %q", tt.input, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ProfileName(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ProfileName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFileStoreProfileLifecycle(t *testing.T) {
	t.Parallel()

	store := FileStore{Dir: filepath.Join(t.TempDir(), "config")}
	current, err := store.CurrentProfile()
	if err != nil {
		t.Fatalf("CurrentProfile on empty store: %v", err)
	}
	if current != "default" {
		t.Fatalf("CurrentProfile on empty store = %q, want default", current)
	}

	profile := Profile{
		CustomerID:       "123-456-7890",
		LoginCustomerID:  "234-567-8901",
		LinkedCustomerID: "345-678-9012",
		Output:           "jsonl",
	}
	if err := store.SaveProfile(" work ", profile); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	got, err := store.LoadProfile("work")
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	want := Profile{
		CustomerID:       "1234567890",
		LoginCustomerID:  "2345678901",
		LinkedCustomerID: "3456789012",
		Output:           "jsonl",
		APIVersion:       DefaultAPIVersion,
	}
	if got != want {
		t.Fatalf("LoadProfile = %#v, want %#v", got, want)
	}
	current, err = store.CurrentProfile()
	if err != nil {
		t.Fatalf("CurrentProfile: %v", err)
	}
	if current != "work" {
		t.Fatalf("first saved profile was not selected: got %q", current)
	}

	if err := store.SaveProfile("other", Profile{CustomerID: "4564564564", APIVersion: "v24"}); err != nil {
		t.Fatalf("SaveProfile other: %v", err)
	}
	current, err = store.CurrentProfile()
	if err != nil || current != "work" {
		t.Fatalf("saving another profile changed current to %q: %v", current, err)
	}
	if err := store.SetCurrentProfile("other"); err != nil {
		t.Fatalf("SetCurrentProfile: %v", err)
	}
	current, err = store.CurrentProfile()
	if err != nil || current != "other" {
		t.Fatalf("CurrentProfile after selection = %q, %v", current, err)
	}

	profiles, err := store.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(profiles) != 2 || profiles["work"] != want || profiles["other"].APIVersion != "v24" {
		t.Fatalf("ListProfiles returned unexpected data: %#v", profiles)
	}
}

func TestFileStoreRejectsInvalidProfileWithoutChangingExistingValue(t *testing.T) {
	t.Parallel()

	store := FileStore{Dir: t.TempDir()}
	original := Profile{CustomerID: "1234567890", APIVersion: "v24"}
	if err := store.SaveProfile("work", original); err != nil {
		t.Fatalf("SaveProfile original: %v", err)
	}

	tests := []struct {
		name    string
		profile Profile
		wantErr string
	}{
		{name: "customer", profile: Profile{CustomerID: "invalid"}, wantErr: "customer ID"},
		{name: "login customer", profile: Profile{LoginCustomerID: "invalid"}, wantErr: "login customer ID"},
		{name: "linked customer", profile: Profile{LinkedCustomerID: "invalid"}, wantErr: "linked customer ID"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := store.SaveProfile("work", tt.profile); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("SaveProfile error = %v, want containing %q", err, tt.wantErr)
			}
			got, err := store.LoadProfile("work")
			if err != nil {
				t.Fatalf("LoadProfile: %v", err)
			}
			if got != original {
				t.Fatalf("invalid save changed profile to %#v, want %#v", got, original)
			}
		})
	}

	if err := store.SaveProfile("../escape", Profile{}); err == nil {
		t.Fatal("SaveProfile accepted unsafe profile name")
	}
	if err := store.SetCurrentProfile("missing"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("SetCurrentProfile missing error = %v", err)
	}
	if _, err := store.LoadProfile("missing"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("LoadProfile missing error = %v", err)
	}
}

func TestFileStoreCredentialLifecycleAndPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are required")
	}

	parent := t.TempDir()
	dir := filepath.Join(parent, "gads")
	store := FileStore{Dir: dir}
	want := Credentials{
		ClientID:       "client",
		ClientSecret:   "secret",
		DeveloperToken: "developer",
		RefreshToken:   "refresh",
	}
	if err := store.SaveCredentials("work", want); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	assertPermissions(t, dir, 0o700)
	assertPermissions(t, store.credentialsPath(), 0o600)
	got, err := store.LoadCredentials("work")
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if got != want {
		t.Fatalf("LoadCredentials = %#v, want %#v", got, want)
	}

	if err := store.DeleteCredentials("work"); err != nil {
		t.Fatalf("DeleteCredentials: %v", err)
	}
	if _, err := store.LoadCredentials("work"); err == nil || !strings.Contains(err.Error(), "do not exist") {
		t.Fatalf("LoadCredentials after delete error = %v", err)
	}
	assertPermissions(t, store.credentialsPath(), 0o600)
	if err := store.SaveCredentials("bad/name", want); err == nil {
		t.Fatal("SaveCredentials accepted unsafe profile name")
	}
}

func TestFileStoreRepairsDirectoryPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are required")
	}

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod temp directory: %v", err)
	}
	store := FileStore{Dir: dir}
	if err := store.SaveProfile("default", Profile{}); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	assertPermissions(t, dir, 0o700)
	assertPermissions(t, store.profilesPath(), 0o600)
}

func TestFileStoreRefusesInsecureFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are required")
	}

	tests := []struct {
		name     string
		filename string
		load     func(FileStore) error
	}{
		{
			name: "profiles", filename: "profiles.json",
			load: func(store FileStore) error { _, err := store.ListProfiles(); return err },
		},
		{
			name: "credentials", filename: "credentials.json",
			load: func(store FileStore) error { _, err := store.LoadCredentials("work"); return err },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tt.filename)
			if err := os.WriteFile(path, []byte(`{"profiles":{}}`), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			err := tt.load(FileStore{Dir: dir})
			if err == nil || !strings.Contains(err.Error(), "refusing insecure permissions 0644") || !strings.Contains(err.Error(), "chmod 600") {
				t.Fatalf("load error = %v", err)
			}
		})
	}
}

func TestFileStoreAtomicReplacementAndCleanup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("replacement semantics differ on Windows")
	}

	store := FileStore{Dir: t.TempDir()}
	if err := store.SaveCredentials("work", Credentials{RefreshToken: "old"}); err != nil {
		t.Fatalf("SaveCredentials old: %v", err)
	}
	oldInfo, err := os.Stat(store.credentialsPath())
	if err != nil {
		t.Fatalf("stat old credentials: %v", err)
	}
	if err := store.SaveCredentials("work", Credentials{RefreshToken: "new"}); err != nil {
		t.Fatalf("SaveCredentials new: %v", err)
	}
	newInfo, err := os.Stat(store.credentialsPath())
	if err != nil {
		t.Fatalf("stat new credentials: %v", err)
	}
	if os.SameFile(oldInfo, newInfo) {
		t.Fatal("credential update modified the existing file instead of atomically replacing it")
	}
	assertNoTemporaryFiles(t, store.Dir)

	before, err := os.ReadFile(store.credentialsPath())
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}
	if err := store.writeJSON(store.credentialsPath(), make(chan int)); err == nil {
		t.Fatal("writeJSON unexpectedly marshaled an unsupported value")
	}
	after, err := os.ReadFile(store.credentialsPath())
	if err != nil {
		t.Fatalf("read credentials after failed write: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("failed write changed the existing credentials file")
	}
	assertNoTemporaryFiles(t, store.Dir)

	blockedTarget := filepath.Join(store.Dir, "blocked")
	if err := os.Mkdir(blockedTarget, 0o700); err != nil {
		t.Fatalf("mkdir blocked target: %v", err)
	}
	if err := store.writeJSON(blockedTarget, map[string]string{"value": "secret"}); err == nil {
		t.Fatal("writeJSON unexpectedly replaced a directory")
	}
	assertNoTemporaryFiles(t, store.Dir)
}

func TestFileStoreMalformedAndNullFiles(t *testing.T) {
	t.Parallel()

	t.Run("malformed", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "profiles.json")
		if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		_, err := (FileStore{Dir: dir}).ListProfiles()
		if err == nil || !strings.Contains(err.Error(), path) {
			t.Fatalf("ListProfiles error = %v, want path context", err)
		}
	})

	t.Run("null profile map", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "profiles.json")
		if err := os.WriteFile(path, []byte(`{"current":"work","profiles":null}`), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		profiles, err := (FileStore{Dir: dir}).ListProfiles()
		if err != nil {
			t.Fatalf("ListProfiles: %v", err)
		}
		if profiles == nil || len(profiles) != 0 {
			t.Fatalf("ListProfiles = %#v, want initialized empty map", profiles)
		}
	})

	t.Run("null credential map remains writable", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "credentials.json")
		if err := os.WriteFile(path, []byte(`{"profiles":null}`), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		store := FileStore{Dir: dir}
		if err := store.SaveCredentials("work", Credentials{RefreshToken: "token"}); err != nil {
			t.Fatalf("SaveCredentials: %v", err)
		}
		var decoded credentialsFile
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read credentials: %v", err)
		}
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("decode credentials: %v", err)
		}
		if decoded.Profiles["work"].RefreshToken != "token" {
			t.Fatalf("saved credentials = %#v", decoded.Profiles)
		}
	})
}

func TestDefaultDir(t *testing.T) {
	dir, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir: %v", err)
	}
	if filepath.Base(dir) != "gads" || !filepath.IsAbs(dir) {
		t.Fatalf("DefaultDir = %q, want absolute path ending in gads", dir)
	}
}

func TestReadJSONPropagatesMissingPath(t *testing.T) {
	t.Parallel()

	var dst map[string]any
	err := (FileStore{Dir: t.TempDir()}).readJSON(filepath.Join(t.TempDir(), "missing"), &dst)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("readJSON error = %v, want os.ErrNotExist", err)
	}
}

func assertPermissions(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("permissions for %s = %04o, want %04o", path, got, want)
	}
}

func assertNoTemporaryFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".gads-*"))
	if err != nil {
		t.Fatalf("glob temporary files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files were not cleaned up: %v", matches)
	}
}
