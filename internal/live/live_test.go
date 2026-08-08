package live_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Aayush9029/gads/internal/api"
	"github.com/Aayush9029/gads/internal/auth"
	"github.com/Aayush9029/gads/internal/config"
)

func TestReadOnlySmoke(t *testing.T) {
	if os.Getenv("GADS_LIVE") != "1" {
		t.Skip("set GADS_LIVE=1 to run the authenticated read-only smoke")
	}
	dir, err := config.DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	store := config.FileStore{Dir: dir}
	profileName := os.Getenv("GADS_PROFILE")
	if profileName == "" {
		profileName, err = store.CurrentProfile()
		if err != nil {
			t.Fatal(err)
		}
	}
	profile, err := store.LoadProfile(profileName)
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := store.LoadCredentials(profileName)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := &api.Client{
		Tokens: &auth.TokenManager{Store: store, Profile: profileName}, Endpoint: config.DefaultAPIEndpoint,
		Version: profile.APIVersion, DeveloperToken: credentials.DeveloperToken,
		LoginCustomerID: profile.LoginCustomerID, LinkedCustomerID: profile.LinkedCustomerID,
	}
	if _, err := client.ListAccessibleCustomers(ctx); err != nil {
		t.Fatalf("list accessible customers: %v", err)
	}
	if profile.CustomerID == "" {
		t.Skip("account discovery passed; set a profile customer ID to run the GAQL smoke")
	}
	response, err := client.Search(ctx, api.SearchOptions{CustomerID: profile.CustomerID, Query: "SELECT customer.id, customer.descriptive_name, customer.currency_code, customer.time_zone, customer.manager, customer.test_account FROM customer LIMIT 1"})
	if err != nil {
		t.Fatalf("customer GAQL smoke: %v", err)
	}
	root, _ := response.Value.(map[string]any)
	results, _ := root["results"].([]any)
	if len(results) == 0 {
		t.Fatal("customer GAQL smoke returned no rows")
	}
}
