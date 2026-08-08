package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Aayush9029/gads/internal/api"
	"github.com/Aayush9029/gads/internal/audit"
	"github.com/Aayush9029/gads/internal/output"
	"github.com/Aayush9029/gads/internal/plan"
)

type resourceDefinition struct {
	Operation string
	From      string
	Fields    string
}

var resources = map[string]resourceDefinition{
	"campaign":     {"campaignOperation", "campaign", "campaign.id, campaign.name, campaign.status, campaign.advertising_channel_type, campaign.bidding_strategy_type, campaign.campaign_budget, campaign.resource_name"},
	"budget":       {"campaignBudgetOperation", "campaign_budget", "campaign_budget.id, campaign_budget.name, campaign_budget.status, campaign_budget.amount_micros, campaign_budget.total_amount_micros, campaign_budget.explicitly_shared, campaign_budget.reference_count, campaign_budget.resource_name"},
	"ad-group":     {"adGroupOperation", "ad_group", "campaign.id, ad_group.id, ad_group.name, ad_group.status, ad_group.type, ad_group.resource_name"},
	"ad":           {"adGroupAdOperation", "ad_group_ad", "campaign.id, ad_group.id, ad_group_ad.ad.id, ad_group_ad.ad.name, ad_group_ad.status, ad_group_ad.ad.type, ad_group_ad.resource_name"},
	"asset":        {"assetOperation", "asset", "asset.id, asset.name, asset.type, asset.source, asset.resource_name"},
	"conversion":   {"conversionActionOperation", "conversion_action", "conversion_action.id, conversion_action.name, conversion_action.status, conversion_action.type, conversion_action.category, conversion_action.resource_name"},
	"app-campaign": {"campaignOperation", "campaign", "campaign.id, campaign.name, campaign.status, campaign.advertising_channel_type, campaign.advertising_channel_sub_type, campaign.resource_name"},
}

func (r *runner) runMutate(ctx context.Context, action string) error {
	switch action {
	case "plan":
		customerID, err := r.requireCustomer()
		if err != nil {
			return err
		}
		operations, err := readOperations(first(r.args.value("file"), "-"), r.deps.Stdin)
		if err != nil {
			return err
		}
		if len(operations) > 10000 {
			return usage(errors.New("Google Ads accepts at most 10,000 mutate operations per request"))
		}
		partial := r.args.bool("partial-failure")
		if partial && hasTemporaryResourceName(operations) {
			return usage(errors.New("partial failure cannot be used with temporary resource dependencies"))
		}
		p, err := plan.New(r.profile.APIVersion, customerID, partial, operations, r.deps.Now())
		if err != nil {
			return err
		}
		return r.finishPlan(ctx, p, "mutate plan")
	case "validate", "execute":
		path := r.args.value("file")
		if path == "" {
			return usage(errors.New("mutate validate and mutate execute require --file PLAN.json"))
		}
		p, err := plan.Read(path)
		if err != nil {
			return err
		}
		if action == "execute" && !r.args.bool("execute") {
			return usage(errors.New("mutate execute requires the explicit --execute flag"))
		}
		return r.sendPlan(ctx, p, action == "validate", "mutate "+action)
	default:
		return usage(fmt.Errorf("unknown mutate subcommand %q", action))
	}
}

func (r *runner) runResource(ctx context.Context, resource, action string) error {
	definition := resources[resource]
	if action == "list" || action == "show" {
		if action == "show" {
			name := first(r.args.value("resource-name"), firstItem(r.args.items))
			if name == "" {
				return usage(errors.New("show requires --resource-name"))
			}
			r.args.values["query"] = fmt.Sprintf("SELECT %s FROM %s WHERE %s.resource_name = '%s'", definition.Fields, definition.From, definition.From, escapeGAQL(name))
		} else {
			r.args.values["query"] = fmt.Sprintf("SELECT %s FROM %s ORDER BY %s.id", definition.Fields, definition.From, definition.From)
		}
		return r.runQuery(ctx, "run")
	}
	if resource == "conversion" && strings.HasPrefix(action, "upload-") {
		return r.planConversionUpload(ctx, action)
	}
	if resource == "asset" && (action == "attach" || action == "detach") {
		return r.planAssetLink(ctx, action)
	}
	if resource == "app-campaign" && action != "create" {
		return r.planAppResource(ctx, action)
	}
	operations, err := r.resourceOperations(resource, action, definition)
	if err != nil {
		return err
	}
	customerID, err := r.requireCustomer()
	if err != nil {
		return err
	}
	p, err := plan.New(r.profile.APIVersion, customerID, r.args.bool("partial-failure"), operations, r.deps.Now())
	if err != nil {
		return err
	}
	return r.finishPlan(ctx, p, resource+" "+action)
}

func (r *runner) resourceOperations(resource, action string, definition resourceDefinition) ([]any, error) {
	operation := map[string]any{}
	switch action {
	case "create":
		body, err := r.resourceBody()
		if err != nil {
			return nil, err
		}
		if resource == "campaign" || resource == "app-campaign" {
			if status, _ := body["status"].(string); status == "ENABLED" {
				return nil, usage(errors.New("new campaigns cannot be enabled; create it paused and use campaign activate separately"))
			}
			body["status"] = "PAUSED"
		}
		if resource == "app-campaign" {
			body["advertisingChannelType"] = "MULTI_CHANNEL"
			if goal := r.args.value("goal"); goal != "" {
				applyAppGoal(body, goal)
			}
		}
		operation[definition.Operation] = map[string]any{"create": body}
	case "update", "update-attestation":
		body, err := r.resourceBody()
		if err != nil {
			return nil, err
		}
		mask := r.args.value("update-mask")
		if mask == "" && action == "update-attestation" {
			mask = "synthetic_content_info"
		}
		if mask == "" {
			return nil, usage(errors.New("update requires --update-mask"))
		}
		operation[definition.Operation] = map[string]any{"update": body, "updateMask": mask}
	case "pause", "activate":
		name := r.args.value("resource-name")
		if name == "" {
			return nil, usage(errors.New("status changes require --resource-name"))
		}
		status := "PAUSED"
		if action == "activate" {
			status = "ENABLED"
		}
		operation[definition.Operation] = map[string]any{"update": map[string]any{"resourceName": name, "status": status}, "updateMask": "status"}
	case "remove":
		name := r.args.value("resource-name")
		if name == "" {
			return nil, usage(errors.New("remove requires --resource-name"))
		}
		if resource == "asset" {
			return nil, usage(errors.New("Google Ads assets cannot be removed; detach the asset association instead"))
		}
		operation[definition.Operation] = map[string]any{"remove": name}
	case "replace":
		if resource != "ad" {
			return nil, usage(fmt.Errorf("%s does not support replace", resource))
		}
		body, err := r.resourceBody()
		if err != nil {
			return nil, err
		}
		old := r.args.value("resource-name")
		if old == "" {
			return nil, usage(errors.New("ad replace requires the old --resource-name"))
		}
		return []any{
			map[string]any{definition.Operation: map[string]any{"create": body}},
			map[string]any{definition.Operation: map[string]any{"update": map[string]any{"resourceName": old, "status": "PAUSED"}, "updateMask": "status"}},
		}, nil
	default:
		return nil, usage(fmt.Errorf("unknown %s subcommand %q", resource, action))
	}
	return []any{operation}, nil
}

func (r *runner) resourceBody() (map[string]any, error) {
	var reader *bytes.Reader
	if inline := r.args.value("body"); inline != "" {
		reader = bytes.NewReader([]byte(inline))
	}
	if path := r.args.value("body-file"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	if reader == nil {
		return nil, usage(errors.New("pass the resource JSON with --body or --body-file"))
	}
	var body map[string]any
	if err := decodeJSON(reader, &body); err != nil {
		return nil, err
	}
	return body, nil
}

func (r *runner) planAssetLink(ctx context.Context, action string) error {
	level := r.args.value("level")
	operations := map[string]string{"customer": "customerAssetOperation", "campaign": "campaignAssetOperation", "ad-group": "adGroupAssetOperation", "asset-group": "assetGroupAssetOperation"}
	operationKey, ok := operations[level]
	if !ok {
		return usage(errors.New("asset attach/detach requires --level customer|campaign|ad-group|asset-group"))
	}
	body, err := r.resourceBody()
	if err != nil {
		return err
	}
	var operation any
	if action == "attach" {
		operation = map[string]any{operationKey: map[string]any{"create": body}}
	} else {
		name, _ := body["resourceName"].(string)
		if name == "" {
			return usage(errors.New("detach body requires resourceName"))
		}
		operation = map[string]any{operationKey: map[string]any{"remove": name}}
	}
	customerID, err := r.requireCustomer()
	if err != nil {
		return err
	}
	p, err := plan.New(r.profile.APIVersion, customerID, false, []any{operation}, r.deps.Now())
	if err != nil {
		return err
	}
	return r.finishPlan(ctx, p, "asset "+action)
}

func (r *runner) planConversionUpload(ctx context.Context, action string) error {
	body, err := r.resourceBody()
	if err != nil {
		return err
	}
	body["partialFailure"] = true
	for _, key := range []string{"conversions", "adjustments"} {
		if values, ok := body[key].([]any); ok && len(values) > 2000 {
			return usage(fmt.Errorf("conversion uploads accept at most 2,000 %s per request", key))
		}
	}
	paths := map[string]string{"upload-clicks": "uploadClickConversions", "upload-calls": "uploadCallConversions", "upload-adjustments": "uploadConversionAdjustments"}
	customerID, err := r.requireCustomer()
	if err != nil {
		return err
	}
	path := fmt.Sprintf("customers/%s:%s", customerID, paths[action])
	p, err := plan.NewRequest(r.profile.APIVersion, customerID, http.MethodPost, path, body, r.deps.Now())
	if err != nil {
		return err
	}
	return r.finishPlan(ctx, p, "conversion "+action)
}

func (r *runner) planAppResource(ctx context.Context, action string) error {
	operationKeys := map[string]string{"ad-group": "adGroupOperation", "ad": "adGroupAdOperation", "audience": "adGroupCriterionOperation", "target": "campaignCriterionOperation"}
	key, ok := operationKeys[action]
	if !ok {
		return usage(fmt.Errorf("unknown app-campaign subcommand %q", action))
	}
	body, err := r.resourceBody()
	if err != nil {
		return err
	}
	customerID, err := r.requireCustomer()
	if err != nil {
		return err
	}
	p, err := plan.New(r.profile.APIVersion, customerID, false, []any{map[string]any{key: map[string]any{"create": body}}}, r.deps.Now())
	if err != nil {
		return err
	}
	return r.finishPlan(ctx, p, "app-campaign "+action)
}

func applyAppGoal(body map[string]any, goal string) {
	goals := map[string]string{
		"installs": "OPTIMIZE_INSTALLS_TARGET_INSTALL_COST", "installs-max": "OPTIMIZE_INSTALLS_WITHOUT_TARGET_INSTALL_COST",
		"installs-action": "OPTIMIZE_IN_APP_CONVERSIONS_TARGET_INSTALL_COST", "actions": "OPTIMIZE_IN_APP_CONVERSIONS_TARGET_CONVERSION_COST",
		"actions-max": "OPTIMIZE_IN_APP_CONVERSIONS_WITHOUT_TARGET_CPA", "prereg": "OPTIMIZE_PRE_REGISTRATION_CONVERSION_VOLUME",
		"value": "OPTIMIZE_RETURN_ON_ADVERTISING_SPEND", "value-max": "OPTIMIZE_TOTAL_VALUE_WITHOUT_TARGET_ROAS",
	}
	if setting, ok := body["appCampaignSetting"].(map[string]any); ok {
		setting["biddingStrategyGoalType"] = first(goals[goal], goal)
	} else {
		body["appCampaignSetting"] = map[string]any{"biddingStrategyGoalType": first(goals[goal], goal)}
	}
}

func (r *runner) finishPlan(ctx context.Context, p plan.Plan, action string) error {
	if r.args.bool("validate") && r.args.bool("execute") {
		return usage(errors.New("choose either --validate or --execute"))
	}
	if path := r.args.value("plan-out"); path != "" {
		if err := plan.Write(path, p); err != nil {
			return err
		}
	}
	if r.args.bool("validate") {
		return r.sendPlan(ctx, p, true, action)
	}
	if r.args.bool("execute") {
		return r.sendPlan(ctx, p, false, action)
	}
	if err := r.auditLogger().Append(audit.Entry{Profile: r.profileName, Action: action, CustomerID: p.CustomerID, PlanHash: p.Hash, Mode: "plan", Result: "planned"}); err != nil {
		return err
	}
	return output.Write(r.deps.Stdout, p, output.JSON)
}

func (r *runner) sendPlan(ctx context.Context, p plan.Plan, validate bool, action string) error {
	if err := p.Verify(); err != nil {
		return err
	}
	if validate && p.LocalValidationOnly {
		return usage(errors.New("this Google Ads operation has no validate_only mode; the local plan is the available validation"))
	}
	if err := r.requireAPI(); err != nil {
		return err
	}
	client := *r.client
	client.Version = p.APIVersion
	mode := "execute"
	if validate {
		mode = "validate"
	}
	var response api.Response
	var err error
	if p.Path != "" {
		body := p.Body
		if validate {
			root, ok := cloneMap(p.Body)
			if !ok {
				return errors.New("action plan body must be a JSON object")
			}
			root["validateOnly"] = true
			body = root
		}
		response, err = client.Do(ctx, api.Request{Method: p.Method, Path: p.Path, Body: body, ReplaySafe: validate})
	} else {
		response, err = client.Mutate(ctx, p.CustomerID, p.Operations, p.PartialFailure, validate)
	}
	entry := audit.Entry{Profile: r.profileName, Action: action, CustomerID: p.CustomerID, PlanHash: p.Hash, Mode: mode, RequestID: response.RequestID, Result: "ok"}
	if err != nil {
		entry.Result = "error"
		entry.Error = err.Error()
	}
	if auditErr := r.auditLogger().Append(entry); auditErr != nil && err == nil {
		return auditErr
	}
	if err != nil {
		return err
	}
	if validate {
		return r.emit(map[string]any{"valid": true, "plan_hash": p.Hash, "request_id": response.RequestID})
	}
	return r.emit(response.Value)
}

func readOperations(path string, stdin io.Reader) ([]any, error) {
	value, err := readJSONValue(path, stdin)
	if err != nil {
		return nil, err
	}
	if operations, ok := value.([]any); ok {
		return operations, nil
	}
	if root, ok := value.(map[string]any); ok {
		if operations, ok := root["operations"].([]any); ok {
			return operations, nil
		}
		if operations, ok := root["mutateOperations"].([]any); ok {
			return operations, nil
		}
	}
	return nil, errors.New("mutation input must be a JSON operation array or an object with operations")
}

func hasTemporaryResourceName(operations []any) bool {
	b, _ := json.Marshal(operations)
	return bytes.Contains(b, []byte("/-"))
}

func cloneMap(value any) (map[string]any, bool) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	var result map[string]any
	if err := decoder.Decode(&result); err != nil {
		return nil, false
	}
	return result, true
}
