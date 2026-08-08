package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Aayush9029/gads/internal/api"
	"github.com/Aayush9029/gads/internal/audit"
	"github.com/Aayush9029/gads/internal/auth"
	"github.com/Aayush9029/gads/internal/commands"
	"github.com/Aayush9029/gads/internal/config"
	"github.com/Aayush9029/gads/internal/output"
)

type Dependencies struct {
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Version   string
	ConfigDir string
	Store     config.Store
	HTTP      *http.Client
	Browser   auth.Browser
	Now       func() time.Time
	Sleep     api.Sleeper
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	var typed *exitError
	if errors.As(err, &typed) {
		return typed.code
	}
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		return 4
	}
	return 1
}

type runner struct {
	deps        Dependencies
	args        arguments
	store       config.Store
	profileName string
	profile     config.Profile
	credentials config.Credentials
	client      *api.Client
	format      output.Format
	configDir   string
}

func Run(ctx context.Context, input []string, deps Dependencies) error {
	if deps.Stdin == nil {
		deps.Stdin = os.Stdin
	}
	if deps.Stdout == nil {
		deps.Stdout = io.Discard
	}
	if deps.Stderr == nil {
		deps.Stderr = io.Discard
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if len(input) == 0 {
		return writeHelp(deps.Stdout, false)
	}
	for _, arg := range input {
		if arg == "--version" {
			_, err := fmt.Fprintf(deps.Stdout, "gads %s\n", deps.Version)
			return err
		}
	}
	if input[0] == "version" {
		_, err := fmt.Fprintf(deps.Stdout, "gads %s\n", deps.Version)
		return err
	}
	if input[0] == "--help" || input[0] == "help" {
		parsed, err := parseArguments(input[1:])
		if err != nil {
			return usage(err)
		}
		return writeHelp(deps.Stdout, parsed.value("format") == "markdown")
	}
	group, action, remaining, err := extractCommand(input)
	if err != nil {
		return usage(err)
	}
	parsed, err := parseArguments(remaining)
	if err != nil {
		return usage(err)
	}
	if value := first(parsed.value("timeout"), os.Getenv("GADS_TIMEOUT")); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return usage(errors.New("--timeout must be a positive duration such as 30s or 2m"))
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, duration)
		defer cancel()
	}
	r := &runner{deps: deps, args: parsed}
	if err := r.prepare(); err != nil {
		return err
	}
	path := group + " " + action
	switch group {
	case "profile":
		return r.runProfile(action)
	case "auth":
		return r.runAuth(ctx, action)
	case "accounts":
		return r.runAccounts(ctx, action)
	case "query":
		return r.runQuery(ctx, action)
	case "fields":
		return r.runFields(ctx, action)
	case "report":
		return r.runReport(ctx, action)
	case "billing":
		return r.runBilling(ctx, action)
	case "change-history":
		return r.runReport(ctx, "change-history")
	case "constants":
		return r.runConstants(ctx, action)
	case "mutate":
		return r.runMutate(ctx, action)
	case "campaign", "budget", "ad-group", "ad", "asset", "conversion", "app-campaign":
		return r.runResource(ctx, group, action)
	default:
		return usage(fmt.Errorf("unknown command %q; run gads help", path))
	}
}

var globalValueFlags = map[string]bool{
	"profile": true, "customer": true, "login-customer": true, "linked-customer": true,
	"output": true, "api-version": true, "endpoint": true, "timeout": true,
}

func extractCommand(input []string) (string, string, []string, error) {
	var command []string
	var remaining []string
	for index := 0; index < len(input); index++ {
		arg := input[index]
		if len(command) < 2 && strings.HasPrefix(arg, "--") {
			keyValue := strings.TrimPrefix(arg, "--")
			key, _, inline := strings.Cut(keyValue, "=")
			if !globalValueFlags[key] {
				return "", "", nil, fmt.Errorf("global flag --%s must follow a command", key)
			}
			remaining = append(remaining, arg)
			if !inline {
				if index+1 >= len(input) {
					return "", "", nil, fmt.Errorf("--%s requires a value", key)
				}
				index++
				remaining = append(remaining, input[index])
			}
			continue
		}
		if len(command) < 2 {
			command = append(command, arg)
		} else {
			remaining = append(remaining, arg)
		}
	}
	if len(command) == 0 {
		return "", "", nil, errors.New("a command is required; run gads help")
	}
	if len(command) == 1 {
		return "", "", nil, fmt.Errorf("%s requires a subcommand; run gads help", command[0])
	}
	return command[0], command[1], remaining, nil
}

func (r *runner) prepare() error {
	dir := r.deps.ConfigDir
	if dir == "" {
		var err error
		dir, err = config.DefaultDir()
		if err != nil {
			return err
		}
	}
	r.configDir = dir
	if r.deps.Store != nil {
		r.store = r.deps.Store
	} else {
		r.store = config.FileStore{Dir: dir}
	}
	name := first(r.args.value("profile"), os.Getenv("GADS_PROFILE"))
	if name == "" {
		current, err := r.store.CurrentProfile()
		if err != nil {
			return err
		}
		name = current
	}
	r.profileName = name
	profile, err := r.store.LoadProfile(name)
	if err == nil {
		r.profile = profile
	}
	r.profile.CustomerID = first(r.args.value("customer"), os.Getenv("GADS_CUSTOMER_ID"), r.profile.CustomerID)
	r.profile.LoginCustomerID = first(r.args.value("login-customer"), os.Getenv("GADS_LOGIN_CUSTOMER_ID"), r.profile.LoginCustomerID)
	r.profile.LinkedCustomerID = first(r.args.value("linked-customer"), os.Getenv("GADS_LINKED_CUSTOMER_ID"), r.profile.LinkedCustomerID)
	for label, value := range map[string]*string{
		"customer ID":        &r.profile.CustomerID,
		"login customer ID":  &r.profile.LoginCustomerID,
		"linked customer ID": &r.profile.LinkedCustomerID,
	} {
		normalized, normalizeErr := config.NormalizeCustomerID(*value)
		if normalizeErr != nil {
			return usage(fmt.Errorf("%s: %w", label, normalizeErr))
		}
		*value = normalized
	}
	r.profile.APIVersion = first(r.args.value("api-version"), os.Getenv("GADS_API_VERSION"), r.profile.APIVersion, config.DefaultAPIVersion)
	format, err := output.ParseFormat(first(r.args.value("output"), os.Getenv("GADS_OUTPUT"), r.profile.Output))
	if err != nil {
		return usage(err)
	}
	r.format = format
	credentials, credentialsErr := r.store.LoadCredentials(name)
	if credentialsErr == nil {
		r.credentials = credentials
	}
	r.credentials.ClientID = first(os.Getenv("GADS_CLIENT_ID"), r.credentials.ClientID)
	r.credentials.ClientSecret = first(os.Getenv("GADS_CLIENT_SECRET"), r.credentials.ClientSecret)
	r.credentials.DeveloperToken = first(os.Getenv("GADS_DEVELOPER_TOKEN"), r.credentials.DeveloperToken)
	r.credentials.RefreshToken = first(os.Getenv("GADS_REFRESH_TOKEN"), r.credentials.RefreshToken)
	tokens := &auth.TokenManager{HTTP: r.deps.HTTP, Credentials: &r.credentials}
	r.client = &api.Client{
		HTTP: r.deps.HTTP, Tokens: tokens, Endpoint: first(r.args.value("endpoint"), os.Getenv("GADS_API_ENDPOINT"), config.DefaultAPIEndpoint),
		Version: r.profile.APIVersion, DeveloperToken: r.credentials.DeveloperToken,
		LoginCustomerID: r.profile.LoginCustomerID, LinkedCustomerID: r.profile.LinkedCustomerID, Sleep: r.deps.Sleep,
	}
	return nil
}

func usage(err error) error { return &exitError{code: 2, err: err} }
func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (r *runner) requireAPI() error {
	if r.credentials.DeveloperToken == "" {
		return &exitError{code: 3, err: errors.New("developer token is missing; run gads auth login or set GADS_DEVELOPER_TOKEN")}
	}
	if r.credentials.RefreshToken == "" {
		return &exitError{code: 3, err: errors.New("OAuth refresh token is missing; run gads auth login or set GADS_REFRESH_TOKEN")}
	}
	return nil
}

func (r *runner) requireCustomer() (string, error) {
	id, err := config.NormalizeCustomerID(r.profile.CustomerID)
	if err != nil {
		return "", usage(err)
	}
	if id == "" {
		return "", usage(errors.New("customer ID is required; pass --customer or set it on the profile"))
	}
	return id, nil
}

func (r *runner) emit(value any) error { return output.Write(r.deps.Stdout, value, r.format) }

func writeHelp(w io.Writer, markdown bool) error {
	if markdown {
		_, err := fmt.Fprintln(w, "# gads command reference\n\nGenerated by `gads help --format markdown`.\n\n| Command | Purpose | Network behavior |\n|---|---|---|")
		if err != nil {
			return err
		}
		for _, command := range commands.SortedRegistry() {
			if _, err := fmt.Fprintf(w, "| `gads %s` | %s | %s |\n", command.Path, command.Description, command.Kind); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := fmt.Fprintln(w, "gads works with Google Ads from the macOS terminal.\n\nUsage:\n  gads <command> <subcommand> [flags]\n\nWrite safety:\n  Create and update commands print a local plan by default.\n  Pass --validate to ask Google to check it without making changes.\n  Pass --execute to make the planned change. New campaigns are always planned as paused.\n\nCommands:"); err != nil {
		return err
	}
	groups := map[string][]commands.Command{}
	for _, command := range commands.Registry {
		group := strings.Split(command.Path, " ")[0]
		groups[group] = append(groups[group], command)
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintf(w, "  %-16s %d subcommands\n", name, len(groups[name])); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, "\nGlobal flags:\n  --profile NAME  --customer ID  --login-customer ID  --output table|json|jsonl|csv\n  --api-version VERSION  --endpoint URL  --timeout 30s\n\nRun `gads help --format markdown` for the full command list.")
	return err
}

func decodeJSON(reader io.Reader, target any) error {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("read JSON: %w", err)
	}
	return nil
}

func readJSONValue(path string, stdin io.Reader) (any, error) {
	var reader io.Reader = stdin
	if path != "" && path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		reader = file
	}
	var value any
	if err := decodeJSON(reader, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (r *runner) auditLogger() audit.Logger {
	return audit.Logger{Path: filepath.Join(r.configDir, "audit.jsonl"), Now: r.deps.Now, Secrets: []string{r.credentials.ClientSecret, r.credentials.DeveloperToken, r.credentials.RefreshToken}}
}
