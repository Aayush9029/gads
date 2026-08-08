package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/Aayush9029/gads/internal/auth"
	"github.com/Aayush9029/gads/internal/config"
)

func (r *runner) runProfile(action string) error {
	switch action {
	case "list":
		profiles, err := r.store.ListProfiles()
		if err != nil {
			return err
		}
		current, err := r.store.CurrentProfile()
		if err != nil {
			return err
		}
		names := make([]string, 0, len(profiles))
		for name := range profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		rows := make([]any, 0, len(names))
		for _, name := range names {
			p := profiles[name]
			rows = append(rows, map[string]any{"name": name, "current": name == current, "customer_id": p.CustomerID, "login_customer_id": p.LoginCustomerID, "api_version": p.APIVersion})
		}
		return r.emit(rows)
	case "show":
		name := first(r.args.value("name"), r.profileName)
		p, err := r.store.LoadProfile(name)
		if err != nil {
			return err
		}
		return r.emit(map[string]any{"name": name, "profile": p})
	case "set":
		name := first(r.args.value("name"), r.profileName)
		p := r.profile
		if value := r.args.value("customer"); value != "" {
			p.CustomerID = value
		}
		if value := r.args.value("login-customer"); value != "" {
			p.LoginCustomerID = value
		}
		if value := r.args.value("linked-customer"); value != "" {
			p.LinkedCustomerID = value
		}
		if value := r.args.value("output"); value != "" {
			p.Output = value
		}
		if value := r.args.value("api-version"); value != "" {
			p.APIVersion = value
		}
		if err := r.store.SaveProfile(name, p); err != nil {
			return err
		}
		return r.emit(map[string]any{"saved": name})
	case "use":
		name := first(r.args.value("name"), firstItem(r.args.items))
		if name == "" {
			return usage(errors.New("profile use requires --name NAME"))
		}
		if err := r.store.SetCurrentProfile(name); err != nil {
			return err
		}
		return r.emit(map[string]any{"current": name})
	default:
		return usage(fmt.Errorf("unknown profile subcommand %q", action))
	}
}

func (r *runner) runAuth(ctx context.Context, action string) error {
	switch action {
	case "login":
		if _, err := r.store.LoadProfile(r.profileName); err != nil {
			if err := r.store.SaveProfile(r.profileName, config.Profile{APIVersion: config.DefaultAPIVersion}); err != nil {
				return err
			}
		}
		flow := auth.LoginFlow{HTTP: r.deps.HTTP, Store: r.store, Browser: r.deps.Browser}
		err := flow.Login(ctx, auth.LoginOptions{
			Profile: r.profileName, ClientID: first(r.args.value("client-id"), r.credentials.ClientID),
			ClientSecret:   first(r.args.value("client-secret"), r.credentials.ClientSecret),
			DeveloperToken: first(r.args.value("developer-token"), r.credentials.DeveloperToken),
		})
		if err != nil {
			return &exitError{code: 3, err: err}
		}
		return r.emit(map[string]any{"profile": r.profileName, "connected": true})
	case "import":
		credentials := config.Credentials{
			ClientID:       first(r.args.value("client-id"), r.credentials.ClientID),
			ClientSecret:   first(r.args.value("client-secret"), r.credentials.ClientSecret),
			DeveloperToken: first(r.args.value("developer-token"), r.credentials.DeveloperToken),
			RefreshToken:   first(r.args.value("refresh-token"), r.credentials.RefreshToken),
		}
		if credentials.ClientID == "" || credentials.ClientSecret == "" || credentials.DeveloperToken == "" || credentials.RefreshToken == "" {
			return usage(errors.New("auth import requires client ID, client secret, developer token, and refresh token"))
		}
		if _, err := r.store.LoadProfile(r.profileName); err != nil {
			if err := r.store.SaveProfile(r.profileName, config.Profile{APIVersion: config.DefaultAPIVersion}); err != nil {
				return err
			}
		}
		if err := r.store.SaveCredentials(r.profileName, credentials); err != nil {
			return err
		}
		return r.emit(map[string]any{"profile": r.profileName, "imported": true})
	case "status":
		credentials, err := r.store.LoadCredentials(r.profileName)
		if err != nil {
			return r.emit(map[string]any{"profile": r.profileName, "connected": false})
		}
		return r.emit(map[string]any{
			"profile": r.profileName, "connected": credentials.RefreshToken != "",
			"client_id_configured": credentials.ClientID != "", "developer_token_configured": credentials.DeveloperToken != "",
		})
	case "logout":
		if err := r.store.DeleteCredentials(r.profileName); err != nil {
			return err
		}
		return r.emit(map[string]any{"profile": r.profileName, "connected": false})
	default:
		return usage(fmt.Errorf("unknown auth subcommand %q", action))
	}
}

func firstItem(items []string) string {
	if len(items) > 0 {
		return items[0]
	}
	return ""
}
