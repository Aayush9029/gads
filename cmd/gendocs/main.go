package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Aayush9029/gads/internal/commands"
)

func main() {
	mustWrite("docs/commands/gads.md", commandReference())
	mustWrite("docs/OFFICIAL_GOOGLE_ADS_DOCS.md", officialNotes())
	mustWrite("docs/OFFICIAL_GOOGLE_ADS_DOC_INDEX.md", officialIndex())
}

func mustWrite(path, contents string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		panic(err)
	}
}

func commandReference() string {
	var b strings.Builder
	b.WriteString("# gads command reference\n\nGenerated from the command registry.\n\n| Command | Purpose | Network behavior |\n|---|---|---|\n")
	for _, command := range commands.SortedRegistry() {
		fmt.Fprintf(&b, "| `gads %s` | %s | %s |\n", command.Path, command.Description, command.Kind)
	}
	b.WriteString("\nWrite commands create a local plan by default. `validate` sends a non-mutating validation request where Google supports it. `mutate` and `action` commands require `--execute` before they can change an account.\n")
	return b.String()
}

func officialNotes() string {
	var b strings.Builder
	b.WriteString(`# Official Google Ads API notes

This document records the API facts used by ` + "`gads`" + `. Google documentation remains authoritative. The matrix below was verified against the v25 reference and official protocol definitions on August 7, 2026.

## Version and support

Google Ads API v25 is the newest released major version. It was released July 22, 2026 and has a tentative sunset in August 2027. v21 through v25 are listed as available, although v21 has a tentative August 2026 sunset. v25.1 was still listed as upcoming when this matrix was verified.

Minor releases update an existing major endpoint without changing its URL. Major releases use a new ` + "`/vN`" + ` namespace and can contain breaking changes. Feature restrictions and policy changes can also happen without a version change.

## Authentication and access

Every API request needs OAuth 2.0 and a developer token. The OAuth scope is ` + "`https://www.googleapis.com/auth/adwords`" + `. User access tokens last about one hour, so installed applications should retain and protect a refresh token. OAuth apps left in Testing can issue refresh tokens that expire after seven days.

Developer-token access levels have separate production rights and sliding 24-hour operation limits:

| Access | Production use | Daily operations |
|---|---|---:|
| Test | Test accounts only | 15,000 |
| Explorer | Restricted production use | 2,880 |
| Basic | Production and test accounts | 15,000 |
| Standard | Production and test accounts | Unlimited for most services |

Explorer Access cannot use account creation, user access, planning, or billing and payment services. Service-specific limits still apply at every access level.

## Customers, managers, and headers

The target customer is the ten-digit ID in the request path. ` + "`login-customer-id`" + ` selects the manager hierarchy used to authorize that request. Directly authorized customers usually do not need it. ` + "`linked-customer-id`" + ` is a separate partner header for approved product-link workflows.

` + "`CustomerService.ListAccessibleCustomers`" + ` lists accounts directly accessible to the OAuth identity. It does not return every descendant. Full account discovery starts at an accessible manager and queries ` + "`customer_client`" + `.

REST requests send ` + "`Authorization: Bearer`" + `, ` + "`developer-token`" + `, and the manager headers when needed. Successful REST requests return a request ID in the response headers. Streaming batches carry their request ID in the JSON body.

## GAQL, fields, search, and streaming

GAQL has one ` + "`FROM`" + ` resource, no explicit joins, no ` + "`OR`" + `, and no offset. Search queries need ` + "`SELECT`" + ` and ` + "`FROM`" + `. GoogleAdsField queries omit ` + "`FROM`" + ` and expose type, compatibility, selectability, filtering, sorting, metrics, and segments.

` + "`GoogleAdsService.Search`" + ` supports ` + "`validateOnly`" + `. ` + "`SearchStream`" + ` does not. v25 Search pages are fixed at 10,000 rows and reject a supplied ` + "`pageSize`" + `. The REST streaming response is an outer JSON array of batches, not JSONL.

Reporting uses the same GAQL services. Metrics cannot be queried directly from a manager account. Daily and other granular data currently has a 37-month retention window; monthly, quarterly, yearly, and high-level data is retained for 11 years.

## Mutations and safety

Most resource services accept create, update, and remove operations. ` + "`GoogleAdsService.Mutate`" + ` combines supported resource types in one request and supports temporary negative resource IDs. Definitions must precede references in the same request.

Mutations are atomic by default. ` + "`partialFailure=true`" + ` commits valid independent operations and returns errors for invalid ones. It should not be used for dependent operations or initial Performance Max asset groups. ` + "`validateOnly=true`" + ` asks Google to check a request without applying it.

Google does not advertise an idempotency key for ordinary mutations. A transport timeout can happen after commit, so executed mutations are not safe to replay blindly. ` + "`gads`" + ` retries reads and validation, but does not automatically retry execution.

Campaign status defaults to enabled in the API. Typed ` + "`gads`" + ` creation always sets it to paused, and activation is a separate plan. Assets are generally immutable. Remove their associations when they should stop serving.

## App Campaigns

App Campaigns use ` + "`MULTI_CHANNEL`" + ` campaigns, a non-shared budget, an App campaign setting, an ad group, and an asset-based ad. The v25 goal enum includes targeted and targetless install, action, and value modes, plus preregistration. The authoritative enum is broader than the five examples in the current creation guide.

Dependent budget, campaign, criterion, ad group, and ad operations can be built as one atomic mutation with temporary resource IDs. New App Campaigns remain paused until a separate activation request.

## Conversions

Conversion actions and custom variables use mutate services. Click, call, and adjustment uploads use specialized services, require ` + "`partialFailure=true`" + `, support validation, and accept at most 2,000 records per request.

Since June 15, 2026, ` + "`UploadClickConversions`" + ` is restricted for developer tokens without recent prior offline upload activity. Google directs new offline and enhanced-lead implementations to the Data Manager API. Conversion uploads should include consent fields when applicable.

## Billing

Billing APIs require a production customer on monthly invoicing and Basic or Standard developer-token access. Billing setup mutations take one operation and do not support ` + "`validateOnly`" + `. Account budget proposals take one operation and do support it. Invoice and payments-account methods are reads.

Billing setup and account budget workflows have approval states and constraints that cannot be inferred locally. Amounts use the account currency in micros. Invoice requests require a serving customer, billing setup resource name, year of 2019 or later, and a month enum.

## Change history

` + "`change_status`" + ` is a broad synchronization feed for the last 90 days and needs a limit of at most 10,000. ` + "`change_event`" + ` provides detailed old and new values for the last 30 days, also with a mandatory limit. Results can lag by about three minutes and do not exactly match the Google Ads UI.

## Quotas and errors

A Search or SearchStream call counts as one operation. Valid follow-on page-token requests do not consume another daily operation. Google accepts at most 10,000 mutate operations in one request, 100 action operations, 2,000 conversion uploads, and one billing or account-budget operation.

REST errors use a canonical status plus ` + "`GoogleAdsFailure`" + `. Each ` + "`GoogleAdsError`" + ` can include a code, message, trigger, and field path. Keep every error and the request ID. Transient reads and validation may retry with bounded exponential backoff and jitter. ` + "`RESOURCE_EXHAUSTED`" + ` usually requires reducing rate or waiting rather than an immediate retry.

## Deprecations and uncertain surfaces

The v25 service protobufs contain no service RPC marked deprecated. Google can still impose unversioned policy changes. New Smart Campaign creation was disabled August 3, 2026. New call-only ad creation is no longer supported. ` + "`AssetGenerationService`" + ` is closed beta, ` + "`MultiPartyAuthReviewService`" + ` is beta, and ` + "`YouTubeVideoUploadService`" + ` is documented for REST and the Python client library only.

The Asset guide still says AssetService only creates assets, while the v25 operation reference permits a narrow update of synthetic-content attestations for eligible asset types. ` + "`gads`" + ` exposes only that narrow typed update and leaves future fields to generic mutation plus Google validation.

## Complete v25 service and operation matrix

Legend: R means read, W means state-changing or externally side-effecting, and S means computation or suggestion without direct account mutation.

The official v25 surface contains 110 services and 169 RPCs: 24 reads, 119 writes, and 26 computations or suggestions. Forty writes do not start with ` + "`Mutate`" + `, so safety classification cannot rely on method names or HTTP verbs.

| Service | Operations |
|---|---|
`)
	for _, service := range commands.OfficialServiceMatrix {
		items := make([]string, 0, len(service.Operations))
		for _, operation := range service.Operations {
			items = append(items, fmt.Sprintf("%s `%s`", operation.Class, operation.Name))
		}
		fmt.Fprintf(&b, "| [%s](https://developers.google.com/google-ads/api/reference/rpc/v25/%s) | %s |\n", service.Name, service.Name, strings.Join(items, "<br>"))
	}
	b.WriteString("\nNo active v25 service RPC was omitted from this inventory. The generic Google Ads mutation escape hatch covers the 64 variants in `MutateOperation`; specialized RPCs still need a typed action or their documented REST method.\n")
	return b.String()
}

func officialIndex() string {
	links := []string{
		"https://developers.google.com/google-ads/api/docs/start",
		"https://developers.google.com/google-ads/api/docs/release-notes",
		"https://developers.google.com/google-ads/api/docs/sunset-dates",
		"https://developers.google.com/google-ads/api/docs/concepts/versioning",
		"https://developers.google.com/google-ads/api/reference/rpc/v25/overview",
		"https://developers.google.com/google-ads/api/reference/rest",
		"https://developers.google.com/google-ads/api/fields/v25/overview",
		"https://developers.google.com/google-ads/api/docs/oauth/overview",
		"https://developers.google.com/google-ads/api/docs/oauth/internals",
		"https://developers.google.com/google-ads/api/docs/oauth/user-authentication",
		"https://developers.google.com/google-ads/api/docs/oauth/credential-management",
		"https://developers.google.com/google-ads/api/docs/oauth/security-requirements",
		"https://developers.google.com/google-ads/api/docs/api-policy/developer-token",
		"https://developers.google.com/google-ads/api/docs/api-policy/access-levels",
		"https://developers.google.com/google-ads/api/rest/auth",
		"https://developers.google.com/google-ads/api/docs/concepts/call-structure",
		"https://developers.google.com/google-ads/api/docs/oauth/access-model",
		"https://developers.google.com/google-ads/api/docs/account-management/listing-accounts",
		"https://developers.google.com/google-ads/api/docs/account-management/get-account-hierarchy",
		"https://developers.google.com/google-ads/api/docs/query/overview",
		"https://developers.google.com/google-ads/api/docs/query/grammar",
		"https://developers.google.com/google-ads/api/docs/query/structure",
		"https://developers.google.com/google-ads/api/docs/query/case-sensitivity",
		"https://developers.google.com/google-ads/api/docs/query/date-ranges",
		"https://developers.google.com/google-ads/api/docs/concepts/field-service",
		"https://developers.google.com/google-ads/api/rest/common/search",
		"https://developers.google.com/google-ads/api/docs/reporting/overview",
		"https://developers.google.com/google-ads/api/docs/reporting/streaming",
		"https://developers.google.com/google-ads/api/docs/reporting/paging",
		"https://developers.google.com/google-ads/api/docs/reporting/segmentation",
		"https://developers.google.com/google-ads/api/docs/reporting/zero-metrics",
		"https://developers.google.com/google-ads/api/docs/mutating/overview",
		"https://developers.google.com/google-ads/api/docs/mutating/service-mutates",
		"https://developers.google.com/google-ads/api/docs/mutating/bulk-mutate",
		"https://developers.google.com/google-ads/api/docs/mutating/best-practices",
		"https://developers.google.com/google-ads/api/docs/best-practices/partial-failures",
		"https://developers.google.com/google-ads/api/docs/campaigns/create-campaigns",
		"https://developers.google.com/google-ads/api/docs/campaigns/budgets/overview",
		"https://developers.google.com/google-ads/api/docs/ads/overview",
		"https://developers.google.com/google-ads/api/docs/assets/overview",
		"https://developers.google.com/google-ads/api/docs/app-campaigns/overview",
		"https://developers.google.com/google-ads/api/docs/app-campaigns/create-campaign",
		"https://developers.google.com/google-ads/api/docs/app-campaigns/create-ad-group",
		"https://developers.google.com/google-ads/api/docs/conversions/overview",
		"https://developers.google.com/google-ads/api/docs/conversions/upload-offline",
		"https://developers.google.com/google-ads/api/docs/conversions/upload-calls",
		"https://developers.google.com/google-ads/api/docs/conversions/adjust-conversions",
		"https://developers.google.com/google-ads/api/docs/billing/overview",
		"https://developers.google.com/google-ads/api/docs/billing/billing-setups",
		"https://developers.google.com/google-ads/api/docs/billing/account-budgets",
		"https://developers.google.com/google-ads/api/docs/billing/invoice",
		"https://developers.google.com/google-ads/api/docs/change-status",
		"https://developers.google.com/google-ads/api/docs/change-event",
		"https://developers.google.com/google-ads/api/data/geotargets",
		"https://developers.google.com/google-ads/api/data/codes-formats",
		"https://developers.google.com/google-ads/api/docs/best-practices/quotas",
		"https://developers.google.com/google-ads/api/docs/best-practices/system-limits",
		"https://developers.google.com/google-ads/api/docs/get-started/handle-errors",
		"https://developers.google.com/google-ads/api/docs/best-practices/understand-api-errors",
		"https://developers.google.com/google-ads/api/docs/deprecations",
	}
	var b strings.Builder
	b.WriteString("# Official Google Ads documentation index\n\n")
	for _, link := range links {
		fmt.Fprintf(&b, "- <%s>\n", link)
	}
	for _, service := range commands.OfficialServiceMatrix {
		fmt.Fprintf(&b, "- <https://developers.google.com/google-ads/api/reference/rpc/v25/%s>\n", service.Name)
		for _, operation := range service.Operations {
			fmt.Fprintf(&b, "  - <https://developers.google.com/google-ads/api/reference/rpc/v25/%s/%s>\n", service.Name, operation.Name)
		}
	}
	return b.String()
}
