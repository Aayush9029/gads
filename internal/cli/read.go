package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/Aayush9029/gads/internal/api"
	"github.com/Aayush9029/gads/internal/plan"
)

func (r *runner) runAccounts(ctx context.Context, action string) error {
	if err := r.requireAPI(); err != nil {
		return err
	}
	switch action {
	case "list":
		response, err := r.client.ListAccessibleCustomers(ctx)
		if err != nil {
			return err
		}
		return r.emit(response.Value)
	case "tree":
		response, err := r.client.ListAccessibleCustomers(ctx)
		if err != nil {
			return err
		}
		root, _ := response.Value.(map[string]any)
		names, _ := root["resourceNames"].([]any)
		maxAccounts, err := r.args.int("max-accounts", 1000)
		if err != nil {
			return usage(err)
		}
		all := make([]any, 0)
		query := "SELECT customer_client.client_customer, customer_client.descriptive_name, customer_client.level, customer_client.manager, customer_client.status FROM customer_client ORDER BY customer_client.level, customer_client.id"
		for _, nameValue := range names {
			name, _ := nameValue.(string)
			id := strings.TrimPrefix(name, "customers/")
			client := *r.client
			client.LoginCustomerID = id
			page, err := searchAll(ctx, &client, api.SearchOptions{CustomerID: id, Query: query}, 10, maxAccounts-len(all))
			if err != nil {
				return err
			}
			all = append(all, extractRows(page)...)
			if len(all) >= maxAccounts {
				break
			}
		}
		return r.emit(all)
	default:
		return usage(fmt.Errorf("unknown accounts subcommand %q", action))
	}
}

func (r *runner) runQuery(ctx context.Context, action string) error {
	if err := r.requireAPI(); err != nil {
		return err
	}
	customerID, err := r.requireCustomer()
	if err != nil {
		return err
	}
	query, err := r.queryText()
	if err != nil {
		return err
	}
	switch action {
	case "validate":
		response, err := r.client.Search(ctx, api.SearchOptions{CustomerID: customerID, Query: query, ValidateOnly: true})
		if err != nil {
			return err
		}
		return r.emit(map[string]any{"valid": true, "request_id": response.RequestID})
	case "run":
		maxRows, err := r.args.int("max-rows", 10000)
		if err != nil {
			return usage(err)
		}
		if maxRows < 1 {
			return usage(errors.New("--max-rows must be positive"))
		}
		if r.args.bool("stream") {
			response, err := r.client.SearchStreamBounded(ctx, customerID, query, maxRows)
			if err != nil {
				return err
			}
			return r.emit(response.Value)
		}
		maxPages, err := r.args.int("max-pages", 10)
		if err != nil {
			return usage(err)
		}
		if maxPages < 1 {
			return usage(errors.New("--max-pages must be positive"))
		}
		value, err := searchAll(ctx, r.client, api.SearchOptions{CustomerID: customerID, Query: query, ReturnTotalResults: r.args.bool("return-total")}, maxPages, maxRows)
		if err != nil {
			return err
		}
		return r.emit(value)
	default:
		return usage(fmt.Errorf("unknown query subcommand %q", action))
	}
}

func (r *runner) queryText() (string, error) {
	if path := r.args.value("file"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(string(b)) == "" {
			return "", usage(errors.New("GAQL file is empty"))
		}
		return string(b), nil
	}
	query := first(r.args.value("query"), strings.Join(r.args.items, " "))
	if strings.TrimSpace(query) == "" {
		return "", usage(errors.New("pass GAQL with --query, --file, or as an argument"))
	}
	return query, nil
}

func (r *runner) runFields(ctx context.Context, action string) error {
	if err := r.requireAPI(); err != nil {
		return err
	}
	var query string
	switch action {
	case "search":
		term := first(r.args.value("query"), strings.Join(r.args.items, " "))
		query = "SELECT name, category, data_type, selectable, filterable, sortable, is_repeated, selectable_with, attribute_resources, metrics, segments"
		if term != "" {
			query += " WHERE name LIKE '%" + escapeGAQL(term) + "%'"
		}
		query += " ORDER BY name"
	case "show":
		name := first(r.args.value("name"), firstItem(r.args.items))
		if name == "" {
			return usage(errors.New("fields show requires a field name"))
		}
		query = "SELECT name, category, data_type, type_url, is_repeated, selectable, filterable, sortable, selectable_with, attribute_resources, metrics, segments WHERE name = '" + escapeGAQL(name) + "'"
	default:
		return usage(fmt.Errorf("unknown fields subcommand %q", action))
	}
	response, err := r.client.SearchFields(ctx, query, "", 0)
	if err != nil {
		return err
	}
	return r.emit(response.Value)
}

var reports = map[string]string{
	"campaigns":      "SELECT campaign.id, campaign.name, campaign.status, campaign.advertising_channel_type, campaign_budget.amount_micros, metrics.impressions, metrics.clicks, metrics.cost_micros, metrics.conversions, metrics.conversions_value FROM campaign WHERE segments.date DURING LAST_30_DAYS ORDER BY metrics.cost_micros DESC",
	"budgets":        "SELECT campaign_budget.id, campaign_budget.name, campaign_budget.status, campaign_budget.amount_micros, campaign_budget.total_amount_micros, campaign_budget.explicitly_shared, campaign_budget.reference_count FROM campaign_budget ORDER BY campaign_budget.name",
	"ad-groups":      "SELECT campaign.id, campaign.name, ad_group.id, ad_group.name, ad_group.status, metrics.impressions, metrics.clicks, metrics.cost_micros, metrics.conversions FROM ad_group WHERE segments.date DURING LAST_30_DAYS ORDER BY metrics.cost_micros DESC",
	"ads":            "SELECT campaign.id, ad_group.id, ad_group_ad.ad.id, ad_group_ad.ad.name, ad_group_ad.status, ad_group_ad.ad.type, metrics.impressions, metrics.clicks, metrics.cost_micros, metrics.conversions FROM ad_group_ad WHERE segments.date DURING LAST_30_DAYS ORDER BY metrics.impressions DESC",
	"assets":         "SELECT asset.id, asset.name, asset.type, asset.resource_name, asset.source FROM asset ORDER BY asset.id",
	"conversions":    "SELECT conversion_action.id, conversion_action.name, conversion_action.status, conversion_action.type, conversion_action.category, conversion_action.primary_for_goal, conversion_action.resource_name FROM conversion_action ORDER BY conversion_action.name",
	"billing":        "SELECT billing_setup.id, billing_setup.status, billing_setup.payments_account, billing_setup.payments_account_info.payments_account_name, billing_setup.start_date_time, billing_setup.end_date_time FROM billing_setup ORDER BY billing_setup.start_date_time DESC",
	"change-history": "SELECT change_event.change_date_time, change_event.change_resource_type, change_event.change_resource_name, change_event.client_type, change_event.user_email, change_event.changed_fields, change_event.old_resource, change_event.new_resource FROM change_event WHERE change_event.change_date_time DURING LAST_30_DAYS ORDER BY change_event.change_date_time DESC LIMIT 10000",
	"geographic":     "SELECT geographic_view.country_criterion_id, geographic_view.location_type, geographic_view.resource_name, metrics.impressions, metrics.clicks, metrics.cost_micros, metrics.conversions FROM geographic_view WHERE segments.date DURING LAST_30_DAYS ORDER BY metrics.impressions DESC",
}

func (r *runner) runReport(ctx context.Context, name string) error {
	query, ok := reports[name]
	if !ok {
		return usage(fmt.Errorf("unknown report %q", name))
	}
	copy := *r
	copy.args.values["query"] = query
	return copy.runQuery(ctx, "run")
}

func (r *runner) runConstants(ctx context.Context, kind string) error {
	if err := r.requireAPI(); err != nil {
		return err
	}
	term := first(r.args.value("query"), strings.Join(r.args.items, " "))
	var query string
	switch kind {
	case "geo":
		query = "SELECT geo_target_constant.id, geo_target_constant.name, geo_target_constant.canonical_name, geo_target_constant.country_code, geo_target_constant.target_type, geo_target_constant.status"
		if term != "" {
			query += " WHERE geo_target_constant.name LIKE '%" + escapeGAQL(term) + "%'"
		}
		query += " ORDER BY geo_target_constant.name"
	case "language":
		query = "SELECT language_constant.id, language_constant.name, language_constant.code, language_constant.targetable"
		if term != "" {
			query += " WHERE language_constant.name LIKE '%" + escapeGAQL(term) + "%'"
		}
		query += " ORDER BY language_constant.name"
	default:
		return usage(fmt.Errorf("unknown constant type %q", kind))
	}
	copy := *r
	copy.args.values["query"] = query
	return copy.runQuery(ctx, "run")
}

func (r *runner) runBilling(ctx context.Context, action string) error {
	if err := r.requireAPI(); err != nil {
		return err
	}
	customerID, err := r.requireCustomer()
	if err != nil {
		return err
	}
	switch action {
	case "setups":
		return r.runReport(ctx, "billing")
	case "accounts":
		response, err := r.client.Do(ctx, api.Request{Method: http.MethodGet, Path: fmt.Sprintf("customers/%s/paymentsAccounts", customerID), ReplaySafe: true})
		if err != nil {
			return err
		}
		return r.emit(response.Value)
	case "invoices":
		billingSetup := r.args.value("billing-setup")
		year := r.args.value("year")
		month := r.args.value("month")
		if billingSetup == "" || year == "" || month == "" {
			return usage(errors.New("billing invoices requires --billing-setup, --year, and --month"))
		}
		if _, err := strconv.Atoi(year); err != nil {
			return usage(errors.New("--year must be an integer"))
		}
		validMonth := map[string]bool{"JANUARY": true, "FEBRUARY": true, "MARCH": true, "APRIL": true, "MAY": true, "JUNE": true, "JULY": true, "AUGUST": true, "SEPTEMBER": true, "OCTOBER": true, "NOVEMBER": true, "DECEMBER": true}
		month = strings.ToUpper(month)
		if !validMonth[month] {
			return usage(errors.New("--month must be a month name such as JANUARY"))
		}
		query := url.Values{"billingSetup": {billingSetup}, "issueYear": {year}, "issueMonth": {month}}
		if r.args.bool("granular") {
			query.Set("includeGranularLevelInvoiceDetails", "true")
		}
		response, err := r.client.Do(ctx, api.Request{Method: http.MethodGet, Path: fmt.Sprintf("customers/%s/invoices", customerID), Query: query, ReplaySafe: true})
		if err != nil {
			return err
		}
		return r.emit(response.Value)
	case "setup-create":
		body, err := r.resourceBody()
		if err != nil {
			return err
		}
		requestBody := map[string]any{"operation": map[string]any{"create": body}}
		p, err := plan.NewLocalRequest(r.profile.APIVersion, customerID, http.MethodPost, fmt.Sprintf("customers/%s/billingSetups:mutate", customerID), requestBody, r.deps.Now())
		if err != nil {
			return err
		}
		return r.finishPlan(ctx, p, "billing setup-create")
	case "setup-cancel":
		name := r.args.value("resource-name")
		if name == "" {
			return usage(errors.New("billing setup-cancel requires --resource-name"))
		}
		requestBody := map[string]any{"operation": map[string]any{"remove": name}}
		p, err := plan.NewLocalRequest(r.profile.APIVersion, customerID, http.MethodPost, fmt.Sprintf("customers/%s/billingSetups:mutate", customerID), requestBody, r.deps.Now())
		if err != nil {
			return err
		}
		return r.finishPlan(ctx, p, "billing setup-cancel")
	case "budget-proposal":
		body, err := r.resourceBody()
		if err != nil {
			return err
		}
		requestBody := map[string]any{"operation": body}
		p, err := plan.NewRequest(r.profile.APIVersion, customerID, http.MethodPost, fmt.Sprintf("customers/%s/accountBudgetProposals:mutate", customerID), requestBody, r.deps.Now())
		if err != nil {
			return err
		}
		return r.finishPlan(ctx, p, "billing budget-proposal")
	default:
		return usage(fmt.Errorf("unknown billing subcommand %q", action))
	}
}

func searchAll(ctx context.Context, client *api.Client, options api.SearchOptions, maxPages, maxRows int) (any, error) {
	combined := map[string]any{"results": []any{}}
	for page := 0; page < maxPages; page++ {
		response, err := client.Search(ctx, options)
		if err != nil {
			return nil, err
		}
		root, _ := response.Value.(map[string]any)
		rows := extractRows(root)
		current := combined["results"].([]any)
		remaining := maxRows - len(current)
		if remaining <= 0 {
			break
		}
		if len(rows) > remaining {
			rows = rows[:remaining]
		}
		combined["results"] = append(current, rows...)
		for _, key := range []string{"totalResultsCount", "fieldMask", "summaryRow"} {
			if value, ok := root[key]; ok {
				combined[key] = value
			}
		}
		token, _ := root["nextPageToken"].(string)
		if token == "" || len(combined["results"].([]any)) >= maxRows {
			break
		}
		options.PageToken = token
	}
	return combined, nil
}

func extractRows(value any) []any {
	root, _ := value.(map[string]any)
	rows, _ := root["results"].([]any)
	return rows
}

func mergeStream(value any, maxRows int) any {
	chunks, ok := value.([]any)
	if !ok {
		return value
	}
	result := map[string]any{"results": []any{}}
	for _, chunk := range chunks {
		root, _ := chunk.(map[string]any)
		rows := extractRows(root)
		current := result["results"].([]any)
		remaining := maxRows - len(current)
		if remaining <= 0 {
			break
		}
		if len(rows) > remaining {
			rows = rows[:remaining]
		}
		result["results"] = append(current, rows...)
		if summary, ok := root["summaryRow"]; ok {
			result["summaryRow"] = summary
		}
	}
	return result
}

func escapeGAQL(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\\", "\\\\"), "'", "\\'")
}
