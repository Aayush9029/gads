# Usage

## Before you start

`gads` needs four pieces of information:

1. A Google Cloud OAuth desktop client ID and client secret.
2. A Google Ads developer token from a manager account's API Center.
3. The Google user who grants access through OAuth.
4. The target customer ID, plus a login customer ID when access passes through a manager.

The OAuth scope is `https://www.googleapis.com/auth/adwords`. Google Ads customer IDs may be entered with or without hyphens. `gads` normalizes them to ten digits before using them in a URL or request header.

## Profiles and credentials

Create a profile for each account or manager path you use:

```bash
gads profile set \
  --name agency \
  --customer 123-456-7890 \
  --login-customer 987-654-3210 \
  --api-version v25 \
  --output table

gads profile use agency
gads profile list
gads profile show --name agency
```

Profiles contain non-secret account settings. OAuth client secrets, refresh tokens, and developer tokens are kept in a separate credentials file. The configuration directory is mode `0700`; files are mode `0600`. `gads` refuses to read a configuration file that is accessible to another user.

Start the browser-based OAuth flow:

```bash
gads auth login \
  --profile agency \
  --client-id "$GADS_CLIENT_ID" \
  --client-secret "$GADS_CLIENT_SECRET" \
  --developer-token "$GADS_DEVELOPER_TOKEN"
```

The callback listener binds to loopback only. The login request asks Google for offline access and uses a random state value. `gads` refreshes access tokens before they expire.

For an existing refresh token, prefer environment variables so the token does not enter shell history:

```bash
export GADS_CLIENT_ID='...'
export GADS_CLIENT_SECRET='...'
export GADS_DEVELOPER_TOKEN='...'
export GADS_REFRESH_TOKEN='...'
gads auth import --profile agency
```

Inspect status without printing secrets:

```bash
gads auth status --profile agency
gads auth logout --profile agency
```

Flags take precedence over environment variables, which take precedence over profile values. Supported account variables are `GADS_PROFILE`, `GADS_CUSTOMER_ID`, `GADS_LOGIN_CUSTOMER_ID`, `GADS_LINKED_CUSTOMER_ID`, `GADS_API_VERSION`, `GADS_OUTPUT`, `GADS_API_ENDPOINT`, and `GADS_TIMEOUT`.

## Direct accounts and manager hierarchies

`CustomerService.ListAccessibleCustomers` returns only accounts directly accessible to the OAuth identity. It does not return every child beneath an accessible manager.

```bash
gads accounts list --output json
gads accounts tree --max-accounts 1000 --output table
```

The target customer belongs in the request path. The login customer selects the manager hierarchy used for authorization. A direct account usually does not need `--login-customer`. The optional linked customer header is separate and is intended for approved partner workflows.

## GAQL

Run a query from an argument or file:

```bash
gads query run \
  --query "SELECT campaign.id, campaign.name, campaign.status FROM campaign ORDER BY campaign.id" \
  --max-pages 10 \
  --max-rows 10000 \
  --output json

gads query run --file report.gaql --stream --max-rows 50000 --output jsonl
```

Paged search is fixed at 10,000 rows per Google page in v25. `gads` does not send `pageSize` because the v25 service rejects it. `--max-pages` and `--max-rows` bound collection. Streaming decodes Google's outer JSON array one batch at a time and stops once the row bound is reached.

Ask Google to validate GAQL without running the report:

```bash
gads query validate --file report.gaql --output json
```

This uses `GoogleAdsService.Search` with `validateOnly`. `SearchStream` does not support validation.

GAQL has one `FROM` resource and no explicit joins, `OR`, offset, or arbitrary SQL functions. Use Google field metadata instead of guessing which metrics and segments work together.

## Field discovery

```bash
gads fields search --query 'campaign.' --output table
gads fields show campaign.status --output json
```

Field results include type, selectability, filtering and sorting support, repeated status, compatible fields, attribute resources, metrics, and segments. Metadata is versioned; pass `--api-version` when inspecting a non-default API family.

## Reports and constants

Built-in reports are ordinary GAQL and remain bounded by the same row and page limits:

```bash
gads report campaigns --output csv
gads report budgets --output table
gads report ad-groups --stream --output jsonl
gads report ads --stream --output jsonl
gads report assets --output json
gads report conversions --output json
gads report billing --output table
gads report change-history --max-rows 10000
gads report geographic --output csv
```

Manager accounts cannot return customer-level metrics. Select a serving customer for metric reports.

Find current location and language constants:

```bash
gads constants geo --query Toronto --output table
gads constants language --query French --output table
```

Google can restructure geographic targets and mark them for removal, so do not keep a permanent local location list.

## Output

Every data command supports `table`, `json`, `jsonl`, and `csv`:

```bash
gads report campaigns --output table
gads report campaigns --output json
gads report campaigns --stream --output jsonl
gads report campaigns --output csv
```

Data goes to stdout. Errors and diagnostics go to stderr. JSON decoding uses `json.Number`, and protobuf int64 values remain decimal strings, so IDs and micros do not pass through floating-point conversion.

## Safe changes

Write commands have three modes:

| Mode | Flag | Google mutation |
|---|---|---|
| Plan | none | No network mutation |
| Validate | `--validate` | No account change |
| Execute | `--execute` | Applies the planned request |

A plan contains the API version, target customer, operations or specialized request, and a SHA-256 hash. The hash excludes the creation timestamp but includes operation order and all request values.

Create a paused campaign plan:

```bash
gads campaign create \
  --body-file campaign.json \
  --plan-out campaign.plan.json
```

`gads` replaces an omitted campaign status with `PAUSED` and rejects `ENABLED` at creation. Validate the same request with Google:

```bash
gads mutate validate --file campaign.plan.json
```

Execute only after reviewing the target customer, operations, and hash:

```bash
gads mutate execute --file campaign.plan.json --execute
```

Activation is a separate operation:

```bash
gads campaign activate \
  --resource-name customers/1234567890/campaigns/111 \
  --plan-out activate.plan.json
```

The same pattern applies to budgets, ad groups, ads, assets, conversion actions, and App Campaign components. Removal is distinct from pausing. Assets themselves cannot generally be deleted; detach the customer, campaign, ad group, or asset group association instead.

Executed mutations are never retried automatically. A timeout can occur after Google committed a request but before the response reached the CLI. Retrying a create or partial-failure batch in that state can duplicate work.

## Generic mutation escape hatch

Pass Google Ads v25 `MutateOperation` JSON as an array:

```json
[
  {
    "campaignOperation": {
      "update": {
        "resourceName": "customers/1234567890/campaigns/111",
        "status": "PAUSED"
      },
      "updateMask": "status"
    }
  }
]
```

Build, validate, and execute the plan:

```bash
gads mutate plan --file operations.json --plan-out pause.plan.json
gads mutate validate --file pause.plan.json
gads mutate execute --file pause.plan.json --execute
```

`--partial-failure` allows independent valid operations to succeed while invalid operations fail. Do not use it for temporary resource-name dependencies. `gads` rejects that combination locally. Google accepts at most 10,000 mutate operations in one request.

## App Campaigns

App Campaign creation always sets `advertisingChannelType` to `MULTI_CHANNEL` and campaign status to `PAUSED`:

```bash
gads app-campaign create \
  --goal installs-max \
  --body-file app-campaign.json \
  --plan-out app-campaign.plan.json
```

Goal aliases are `installs`, `installs-max`, `installs-action`, `actions`, `actions-max`, `prereg`, `value`, and `value-max`. Google validation remains authoritative for the goal's subtype, bidding target, conversion actions, app store, and required assets.

Plan the remaining resources with `app-campaign ad-group`, `app-campaign ad`, `app-campaign audience`, and `app-campaign target`. Use temporary negative resource IDs in one atomic generic plan when resources depend on each other.

## Conversions

Conversion actions use the normal mutate pipeline. Upload requests use the specialized services:

```bash
gads conversion upload-clicks --body-file clicks.json --plan-out clicks.plan.json
gads conversion upload-calls --body-file calls.json --validate
gads conversion upload-adjustments --body-file adjustments.json --execute
```

Google requires `partialFailure=true` for conversion uploads and limits each request to 2,000 conversions. Include consent data where applicable. As of June 15, 2026, developer tokens without prior offline conversion activity are restricted from `UploadClickConversions`; Google directs new offline and enhanced-lead implementations to the Data Manager API.

## Billing

Billing APIs require a production account on monthly invoicing and Basic or Standard developer-token access. Explorer Access cannot use payment, billing setup, account budget proposal, or invoice services.

```bash
gads billing setups --output table
gads billing accounts --output json
gads billing invoices \
  --billing-setup customers/1234567890/billingSetups/222 \
  --year 2026 \
  --month JULY \
  --granular
```

Billing setup mutations accept one operation and do not support Google's `validateOnly`. `gads billing setup-create` and `gads billing setup-cancel` therefore provide a local plan and an explicit execute gate, but reject `--validate`.

Account budget proposals support Google validation:

```bash
gads billing budget-proposal --body-file proposal-operation.json --validate
```

Amounts use the account currency in micros. Read the customer currency before interpreting a budget.

## Audit log

Planning, validation, and execution append JSONL records to the profile configuration directory. Records contain the profile, customer, plan hash, mode, outcome, and request ID. Known credentials and authorization fields are redacted. The audit file is mode `0600`.

## Timeouts, retries, and cancellation

Press Control-C to stop OAuth, paging, streaming, retry waits, and API calls. Add a deadline when needed:

```bash
gads query run --file report.gaql --timeout 2m
```

Reads and Google validation may retry transient server failures with bounded exponential backoff and jitter. Executed writes do not retry. API errors include every decoded Google Ads error, field location, trigger when safe, and request ID.

## Live tests

Automated tests use injected transports and local fake servers. A live smoke is opt-in and read-only. Release publication is blocked until an authenticated account-discovery and GAQL smoke succeeds with real credentials.
