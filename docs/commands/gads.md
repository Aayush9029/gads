# gads command reference

Generated from the command registry.

| Command | Purpose | Network behavior |
|---|---|---|
| `gads accounts list` | List accounts directly accessible to the OAuth identity | read |
| `gads accounts tree` | Walk customer and manager account relationships | read |
| `gads ad activate` | Plan activating an ad | mutate |
| `gads ad create` | Plan ad creation | mutate |
| `gads ad list` | List ads | read |
| `gads ad pause` | Plan pausing an ad | mutate |
| `gads ad remove` | Plan removing an ad | mutate |
| `gads ad replace` | Plan an ad creative replacement | mutate |
| `gads ad show` | Show one ad | read |
| `gads ad-group activate` | Plan activating an ad group | mutate |
| `gads ad-group create` | Plan ad group creation | mutate |
| `gads ad-group list` | List ad groups | read |
| `gads ad-group pause` | Plan pausing an ad group | mutate |
| `gads ad-group remove` | Plan removing an ad group | mutate |
| `gads ad-group show` | Show one ad group | read |
| `gads ad-group update` | Plan an ad group update | mutate |
| `gads app-campaign ad` | Plan an App campaign ad | mutate |
| `gads app-campaign ad-group` | Plan an App campaign ad group | mutate |
| `gads app-campaign audience` | Plan an App engagement audience criterion | mutate |
| `gads app-campaign create` | Plan a paused App campaign | mutate |
| `gads app-campaign target` | Plan App campaign geo or language targeting | mutate |
| `gads asset attach` | Plan an asset association | mutate |
| `gads asset create` | Plan asset creation | mutate |
| `gads asset detach` | Plan removal of an asset association | mutate |
| `gads asset list` | List assets | read |
| `gads asset show` | Show one asset | read |
| `gads asset update-attestation` | Plan an eligible synthetic-content attestation update | mutate |
| `gads auth import` | Store an existing OAuth refresh token | local |
| `gads auth login` | Connect a Google account with OAuth | local |
| `gads auth logout` | Remove credentials for a profile | local |
| `gads auth status` | Show credential status without revealing secrets | local |
| `gads billing accounts` | List payments accounts | read |
| `gads billing budget-proposal` | Plan an account budget proposal | action |
| `gads billing invoices` | List invoices for a billing setup | read |
| `gads billing setup-cancel` | Plan cancellation of an eligible billing setup | action |
| `gads billing setup-create` | Plan a monthly-invoicing billing setup | action |
| `gads billing setups` | List billing setups | read |
| `gads budget create` | Plan campaign budget creation | mutate |
| `gads budget list` | List campaign budgets | read |
| `gads budget remove` | Plan removing a campaign budget | mutate |
| `gads budget show` | Show one campaign budget | read |
| `gads budget update` | Plan a campaign budget update | mutate |
| `gads campaign activate` | Plan activating a campaign | mutate |
| `gads campaign create` | Plan a paused campaign creation | mutate |
| `gads campaign list` | List campaigns | read |
| `gads campaign pause` | Plan pausing a campaign | mutate |
| `gads campaign remove` | Plan removing a campaign | mutate |
| `gads campaign show` | Show one campaign | read |
| `gads campaign update` | Plan a campaign update | mutate |
| `gads change-history list` | List recent account changes | read |
| `gads constants geo` | Search geographic target constants | read |
| `gads constants language` | Search language constants | read |
| `gads conversion create` | Plan conversion action creation | mutate |
| `gads conversion list` | List conversion actions | read |
| `gads conversion update` | Plan a conversion action update | mutate |
| `gads conversion upload-adjustments` | Plan conversion adjustments | action |
| `gads conversion upload-calls` | Plan an offline call conversion upload | action |
| `gads conversion upload-clicks` | Plan an offline click conversion upload | action |
| `gads fields search` | Search GoogleAdsField metadata | read |
| `gads fields show` | Show metadata for one Google Ads field | read |
| `gads mutate execute` | Execute a verified plan with an explicit flag | mutate |
| `gads mutate plan` | Create a deterministic local mutation plan | local |
| `gads mutate validate` | Validate a plan with Google without changing an account | validate |
| `gads profile list` | List profiles | local |
| `gads profile set` | Create or update a profile | local |
| `gads profile show` | Show a profile | local |
| `gads profile use` | Select the default profile | local |
| `gads query run` | Run GAQL with bounded paging or streaming | read |
| `gads query validate` | Ask Google to validate GAQL without returning data | validate |
| `gads report ad-groups` | Report ad group performance | read |
| `gads report ads` | Report ad performance | read |
| `gads report assets` | Report asset performance | read |
| `gads report billing` | Report billing setup and payment accounts | read |
| `gads report budgets` | Report campaign budget settings | read |
| `gads report campaigns` | Report campaign performance | read |
| `gads report change-history` | Report recent account changes | read |
| `gads report conversions` | Report conversion actions and performance | read |
| `gads report geographic` | Report geographic performance | read |

Write commands create a local plan by default. `validate` sends a non-mutating validation request where Google supports it. `mutate` and `action` commands require `--execute` before they can change an account.
