# Official Google Ads API notes

This document records the API facts used by `gads`. Google documentation remains authoritative. The matrix below was verified against the v25 reference and official protocol definitions on August 7, 2026.

## Version and support

Google Ads API v25 is the newest released major version. It was released July 22, 2026 and has a tentative sunset in August 2027. v21 through v25 are listed as available, although v21 has a tentative August 2026 sunset. v25.1 was still listed as upcoming when this matrix was verified.

Minor releases update an existing major endpoint without changing its URL. Major releases use a new `/vN` namespace and can contain breaking changes. Feature restrictions and policy changes can also happen without a version change.

## Authentication and access

Every API request needs OAuth 2.0 and a developer token. The OAuth scope is `https://www.googleapis.com/auth/adwords`. User access tokens last about one hour, so installed applications should retain and protect a refresh token. OAuth apps left in Testing can issue refresh tokens that expire after seven days.

Developer-token access levels have separate production rights and sliding 24-hour operation limits:

| Access | Production use | Daily operations |
|---|---|---:|
| Test | Test accounts only | 15,000 |
| Explorer | Restricted production use | 2,880 |
| Basic | Production and test accounts | 15,000 |
| Standard | Production and test accounts | Unlimited for most services |

Explorer Access cannot use account creation, user access, planning, or billing and payment services. Service-specific limits still apply at every access level.

## Customers, managers, and headers

The target customer is the ten-digit ID in the request path. `login-customer-id` selects the manager hierarchy used to authorize that request. Directly authorized customers usually do not need it. `linked-customer-id` is a separate partner header for approved product-link workflows.

`CustomerService.ListAccessibleCustomers` lists accounts directly accessible to the OAuth identity. It does not return every descendant. Full account discovery starts at an accessible manager and queries `customer_client`.

REST requests send `Authorization: Bearer`, `developer-token`, and the manager headers when needed. Successful REST requests return a request ID in the response headers. Streaming batches carry their request ID in the JSON body.

## GAQL, fields, search, and streaming

GAQL has one `FROM` resource, no explicit joins, no `OR`, and no offset. Search queries need `SELECT` and `FROM`. GoogleAdsField queries omit `FROM` and expose type, compatibility, selectability, filtering, sorting, metrics, and segments.

`GoogleAdsService.Search` supports `validateOnly`. `SearchStream` does not. v25 Search pages are fixed at 10,000 rows and reject a supplied `pageSize`. The REST streaming response is an outer JSON array of batches, not JSONL.

Reporting uses the same GAQL services. Metrics cannot be queried directly from a manager account. Daily and other granular data currently has a 37-month retention window; monthly, quarterly, yearly, and high-level data is retained for 11 years.

## Mutations and safety

Most resource services accept create, update, and remove operations. `GoogleAdsService.Mutate` combines supported resource types in one request and supports temporary negative resource IDs. Definitions must precede references in the same request.

Mutations are atomic by default. `partialFailure=true` commits valid independent operations and returns errors for invalid ones. It should not be used for dependent operations or initial Performance Max asset groups. `validateOnly=true` asks Google to check a request without applying it.

Google does not advertise an idempotency key for ordinary mutations. A transport timeout can happen after commit, so executed mutations are not safe to replay blindly. `gads` retries reads and validation, but does not automatically retry execution.

Campaign status defaults to enabled in the API. Typed `gads` creation always sets it to paused, and activation is a separate plan. Assets are generally immutable. Remove their associations when they should stop serving.

## App Campaigns

App Campaigns use `MULTI_CHANNEL` campaigns, a non-shared budget, an App campaign setting, an ad group, and an asset-based ad. The v25 goal enum includes targeted and targetless install, action, and value modes, plus preregistration. The authoritative enum is broader than the five examples in the current creation guide.

Dependent budget, campaign, criterion, ad group, and ad operations can be built as one atomic mutation with temporary resource IDs. New App Campaigns remain paused until a separate activation request.

## Conversions

Conversion actions and custom variables use mutate services. Click, call, and adjustment uploads use specialized services, require `partialFailure=true`, support validation, and accept at most 2,000 records per request.

Since June 15, 2026, `UploadClickConversions` is restricted for developer tokens without recent prior offline upload activity. Google directs new offline and enhanced-lead implementations to the Data Manager API. Conversion uploads should include consent fields when applicable.

## Billing

Billing APIs require a production customer on monthly invoicing and Basic or Standard developer-token access. Billing setup mutations take one operation and do not support `validateOnly`. Account budget proposals take one operation and do support it. Invoice and payments-account methods are reads.

Billing setup and account budget workflows have approval states and constraints that cannot be inferred locally. Amounts use the account currency in micros. Invoice requests require a serving customer, billing setup resource name, year of 2019 or later, and a month enum.

## Change history

`change_status` is a broad synchronization feed for the last 90 days and needs a limit of at most 10,000. `change_event` provides detailed old and new values for the last 30 days, also with a mandatory limit. Results can lag by about three minutes and do not exactly match the Google Ads UI.

## Quotas and errors

A Search or SearchStream call counts as one operation. Valid follow-on page-token requests do not consume another daily operation. Google accepts at most 10,000 mutate operations in one request, 100 action operations, 2,000 conversion uploads, and one billing or account-budget operation.

REST errors use a canonical status plus `GoogleAdsFailure`. Each `GoogleAdsError` can include a code, message, trigger, and field path. Keep every error and the request ID. Transient reads and validation may retry with bounded exponential backoff and jitter. `RESOURCE_EXHAUSTED` usually requires reducing rate or waiting rather than an immediate retry.

## Deprecations and uncertain surfaces

The v25 service protobufs contain no service RPC marked deprecated. Google can still impose unversioned policy changes. New Smart Campaign creation was disabled August 3, 2026. New call-only ad creation is no longer supported. `AssetGenerationService` is closed beta, `MultiPartyAuthReviewService` is beta, and `YouTubeVideoUploadService` is documented for REST and the Python client library only.

The Asset guide still says AssetService only creates assets, while the v25 operation reference permits a narrow update of synthetic-content attestations for eligible asset types. `gads` exposes only that narrow typed update and leaves future fields to generic mutation plus Google validation.

## Complete v25 service and operation matrix

Legend: R means read, W means state-changing or externally side-effecting, and S means computation or suggestion without direct account mutation.

The official v25 surface contains 110 services and 169 RPCs: 24 reads, 119 writes, and 26 computations or suggestions. Forty writes do not start with `Mutate`, so safety classification cannot rely on method names or HTTP verbs.

| Service | Operations |
|---|---|
| [AccountBudgetProposalService](https://developers.google.com/google-ads/api/reference/rpc/v25/AccountBudgetProposalService) | W `MutateAccountBudgetProposal` |
| [AccountLinkService](https://developers.google.com/google-ads/api/reference/rpc/v25/AccountLinkService) | W `CreateAccountLink`<br>W `MutateAccountLink` |
| [AdGroupAdLabelService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdGroupAdLabelService) | W `MutateAdGroupAdLabels` |
| [AdGroupAdService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdGroupAdService) | W `MutateAdGroupAds`<br>W `RemoveAutomaticallyCreatedAssets` |
| [AdGroupAssetService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdGroupAssetService) | W `MutateAdGroupAssets` |
| [AdGroupAssetSetService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdGroupAssetSetService) | W `MutateAdGroupAssetSets` |
| [AdGroupBidModifierService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdGroupBidModifierService) | W `MutateAdGroupBidModifiers` |
| [AdGroupCriterionCustomizerService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdGroupCriterionCustomizerService) | W `MutateAdGroupCriterionCustomizers` |
| [AdGroupCriterionLabelService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdGroupCriterionLabelService) | W `MutateAdGroupCriterionLabels` |
| [AdGroupCriterionService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdGroupCriterionService) | W `MutateAdGroupCriteria` |
| [AdGroupCustomizerService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdGroupCustomizerService) | W `MutateAdGroupCustomizers` |
| [AdGroupLabelService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdGroupLabelService) | W `MutateAdGroupLabels` |
| [AdGroupService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdGroupService) | W `MutateAdGroups` |
| [AdParameterService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdParameterService) | W `MutateAdParameters` |
| [AdService](https://developers.google.com/google-ads/api/reference/rpc/v25/AdService) | W `MutateAds` |
| [AssetGenerationService](https://developers.google.com/google-ads/api/reference/rpc/v25/AssetGenerationService) | S `GenerateText`<br>S `GenerateImages` |
| [AssetGroupAssetService](https://developers.google.com/google-ads/api/reference/rpc/v25/AssetGroupAssetService) | W `MutateAssetGroupAssets` |
| [AssetGroupListingGroupFilterService](https://developers.google.com/google-ads/api/reference/rpc/v25/AssetGroupListingGroupFilterService) | W `MutateAssetGroupListingGroupFilters` |
| [AssetGroupService](https://developers.google.com/google-ads/api/reference/rpc/v25/AssetGroupService) | W `MutateAssetGroups` |
| [AssetGroupSignalService](https://developers.google.com/google-ads/api/reference/rpc/v25/AssetGroupSignalService) | W `MutateAssetGroupSignals` |
| [AssetService](https://developers.google.com/google-ads/api/reference/rpc/v25/AssetService) | W `MutateAssets` |
| [AssetSetAssetService](https://developers.google.com/google-ads/api/reference/rpc/v25/AssetSetAssetService) | W `MutateAssetSetAssets` |
| [AssetSetService](https://developers.google.com/google-ads/api/reference/rpc/v25/AssetSetService) | W `MutateAssetSets` |
| [AudienceInsightsService](https://developers.google.com/google-ads/api/reference/rpc/v25/AudienceInsightsService) | R `ListAudienceInsightsAttributes`<br>R `ListInsightsEligibleDates`<br>S `GenerateInsightsFinderReport`<br>S `GenerateAudienceCompositionInsights`<br>S `GenerateAudienceDefinition`<br>S `GenerateSuggestedTargetingInsights`<br>S `GenerateAudienceOverlapInsights`<br>S `GenerateTargetingSuggestionMetrics` |
| [AudienceService](https://developers.google.com/google-ads/api/reference/rpc/v25/AudienceService) | W `MutateAudiences` |
| [AutomaticallyCreatedAssetRemovalService](https://developers.google.com/google-ads/api/reference/rpc/v25/AutomaticallyCreatedAssetRemovalService) | W `RemoveCampaignAutomaticallyCreatedAsset` |
| [BatchJobService](https://developers.google.com/google-ads/api/reference/rpc/v25/BatchJobService) | R `ListBatchJobResults`<br>W `MutateBatchJob`<br>W `RunBatchJob`<br>W `AddBatchJobOperations` |
| [BenchmarksService](https://developers.google.com/google-ads/api/reference/rpc/v25/BenchmarksService) | R `ListBenchmarksAvailableDates`<br>R `ListBenchmarksLocations`<br>R `ListBenchmarksProducts`<br>R `ListBenchmarksSources`<br>S `GenerateBenchmarksMetrics` |
| [BiddingDataExclusionService](https://developers.google.com/google-ads/api/reference/rpc/v25/BiddingDataExclusionService) | W `MutateBiddingDataExclusions` |
| [BiddingSeasonalityAdjustmentService](https://developers.google.com/google-ads/api/reference/rpc/v25/BiddingSeasonalityAdjustmentService) | W `MutateBiddingSeasonalityAdjustments` |
| [BiddingStrategyService](https://developers.google.com/google-ads/api/reference/rpc/v25/BiddingStrategyService) | W `MutateBiddingStrategies` |
| [BillingSetupService](https://developers.google.com/google-ads/api/reference/rpc/v25/BillingSetupService) | W `MutateBillingSetup` |
| [BrandSuggestionService](https://developers.google.com/google-ads/api/reference/rpc/v25/BrandSuggestionService) | S `SuggestBrands` |
| [CampaignAssetService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignAssetService) | W `MutateCampaignAssets` |
| [CampaignAssetSetService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignAssetSetService) | W `MutateCampaignAssetSets` |
| [CampaignBidModifierService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignBidModifierService) | W `MutateCampaignBidModifiers` |
| [CampaignBudgetService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignBudgetService) | W `MutateCampaignBudgets` |
| [CampaignConversionGoalService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignConversionGoalService) | W `MutateCampaignConversionGoals` |
| [CampaignCriterionService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignCriterionService) | W `MutateCampaignCriteria` |
| [CampaignCustomizerService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignCustomizerService) | W `MutateCampaignCustomizers` |
| [CampaignDraftService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignDraftService) | R `ListCampaignDraftAsyncErrors`<br>W `MutateCampaignDrafts`<br>W `PromoteCampaignDraft` |
| [CampaignGoalConfigService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignGoalConfigService) | W `MutateCampaignGoalConfigs` |
| [CampaignGroupService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignGroupService) | W `MutateCampaignGroups` |
| [CampaignLabelService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignLabelService) | W `MutateCampaignLabels` |
| [CampaignService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignService) | W `MutateCampaigns`<br>W `EnablePMaxBrandGuidelines` |
| [CampaignSharedSetService](https://developers.google.com/google-ads/api/reference/rpc/v25/CampaignSharedSetService) | W `MutateCampaignSharedSets` |
| [ContentCreatorInsightsService](https://developers.google.com/google-ads/api/reference/rpc/v25/ContentCreatorInsightsService) | S `GenerateCreatorInsights`<br>S `GenerateTrendingInsights` |
| [ConversionActionService](https://developers.google.com/google-ads/api/reference/rpc/v25/ConversionActionService) | W `MutateConversionActions` |
| [ConversionAdjustmentUploadService](https://developers.google.com/google-ads/api/reference/rpc/v25/ConversionAdjustmentUploadService) | W `UploadConversionAdjustments` |
| [ConversionCustomVariableService](https://developers.google.com/google-ads/api/reference/rpc/v25/ConversionCustomVariableService) | W `MutateConversionCustomVariables` |
| [ConversionGoalCampaignConfigService](https://developers.google.com/google-ads/api/reference/rpc/v25/ConversionGoalCampaignConfigService) | W `MutateConversionGoalCampaignConfigs` |
| [ConversionUploadService](https://developers.google.com/google-ads/api/reference/rpc/v25/ConversionUploadService) | W `UploadClickConversions`<br>W `UploadCallConversions` |
| [ConversionValueRuleService](https://developers.google.com/google-ads/api/reference/rpc/v25/ConversionValueRuleService) | W `MutateConversionValueRules` |
| [ConversionValueRuleSetService](https://developers.google.com/google-ads/api/reference/rpc/v25/ConversionValueRuleSetService) | W `MutateConversionValueRuleSets` |
| [CustomAudienceService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomAudienceService) | W `MutateCustomAudiences` |
| [CustomConversionGoalService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomConversionGoalService) | W `MutateCustomConversionGoals` |
| [CustomInterestService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomInterestService) | W `MutateCustomInterests` |
| [CustomerAssetService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomerAssetService) | W `MutateCustomerAssets` |
| [CustomerAssetSetService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomerAssetSetService) | W `MutateCustomerAssetSets` |
| [CustomerClientLinkService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomerClientLinkService) | W `MutateCustomerClientLink` |
| [CustomerConversionGoalService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomerConversionGoalService) | W `MutateCustomerConversionGoals` |
| [CustomerCustomizerService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomerCustomizerService) | W `MutateCustomerCustomizers` |
| [CustomerLabelService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomerLabelService) | W `MutateCustomerLabels` |
| [CustomerManagerLinkService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomerManagerLinkService) | W `MutateCustomerManagerLink`<br>W `MoveManagerLink` |
| [CustomerNegativeCriterionService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomerNegativeCriterionService) | W `MutateCustomerNegativeCriteria` |
| [CustomerService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomerService) | R `ListAccessibleCustomers`<br>W `MutateCustomer`<br>W `CreateCustomerClient` |
| [CustomerSkAdNetworkConversionValueSchemaService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomerSkAdNetworkConversionValueSchemaService) | W `MutateCustomerSkAdNetworkConversionValueSchema` |
| [CustomerUserAccessInvitationService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomerUserAccessInvitationService) | W `MutateCustomerUserAccessInvitation` |
| [CustomerUserAccessService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomerUserAccessService) | W `MutateCustomerUserAccess` |
| [CustomizerAttributeService](https://developers.google.com/google-ads/api/reference/rpc/v25/CustomizerAttributeService) | W `MutateCustomizerAttributes` |
| [DataLinkService](https://developers.google.com/google-ads/api/reference/rpc/v25/DataLinkService) | W `CreateDataLink`<br>W `RemoveDataLink`<br>W `UpdateDataLink` |
| [ExperimentArmService](https://developers.google.com/google-ads/api/reference/rpc/v25/ExperimentArmService) | W `MutateExperimentArms` |
| [ExperimentService](https://developers.google.com/google-ads/api/reference/rpc/v25/ExperimentService) | R `ListExperimentAsyncErrors`<br>W `MutateExperiments`<br>W `EndExperiment`<br>W `GraduateExperiment`<br>W `ScheduleExperiment`<br>W `PromoteExperiment` |
| [GeoTargetConstantService](https://developers.google.com/google-ads/api/reference/rpc/v25/GeoTargetConstantService) | S `SuggestGeoTargetConstants` |
| [GoalService](https://developers.google.com/google-ads/api/reference/rpc/v25/GoalService) | W `MutateGoals` |
| [GoogleAdsFieldService](https://developers.google.com/google-ads/api/reference/rpc/v25/GoogleAdsFieldService) | R `GetGoogleAdsField`<br>R `SearchGoogleAdsFields` |
| [GoogleAdsService](https://developers.google.com/google-ads/api/reference/rpc/v25/GoogleAdsService) | R `Search`<br>R `SearchStream`<br>W `Mutate` |
| [IdentityVerificationService](https://developers.google.com/google-ads/api/reference/rpc/v25/IdentityVerificationService) | R `GetIdentityVerification`<br>W `StartIdentityVerification` |
| [IncentiveService](https://developers.google.com/google-ads/api/reference/rpc/v25/IncentiveService) | R `FetchIncentive`<br>W `ApplyIncentive` |
| [InvoiceService](https://developers.google.com/google-ads/api/reference/rpc/v25/InvoiceService) | R `ListInvoices` |
| [KeywordPlanAdGroupKeywordService](https://developers.google.com/google-ads/api/reference/rpc/v25/KeywordPlanAdGroupKeywordService) | W `MutateKeywordPlanAdGroupKeywords` |
| [KeywordPlanAdGroupService](https://developers.google.com/google-ads/api/reference/rpc/v25/KeywordPlanAdGroupService) | W `MutateKeywordPlanAdGroups` |
| [KeywordPlanCampaignKeywordService](https://developers.google.com/google-ads/api/reference/rpc/v25/KeywordPlanCampaignKeywordService) | W `MutateKeywordPlanCampaignKeywords` |
| [KeywordPlanCampaignService](https://developers.google.com/google-ads/api/reference/rpc/v25/KeywordPlanCampaignService) | W `MutateKeywordPlanCampaigns` |
| [KeywordPlanIdeaService](https://developers.google.com/google-ads/api/reference/rpc/v25/KeywordPlanIdeaService) | S `GenerateKeywordIdeas`<br>S `GenerateKeywordHistoricalMetrics`<br>S `GenerateAdGroupThemes`<br>S `GenerateKeywordForecastMetrics` |
| [KeywordPlanService](https://developers.google.com/google-ads/api/reference/rpc/v25/KeywordPlanService) | W `MutateKeywordPlans` |
| [KeywordThemeConstantService](https://developers.google.com/google-ads/api/reference/rpc/v25/KeywordThemeConstantService) | S `SuggestKeywordThemeConstants` |
| [LabelService](https://developers.google.com/google-ads/api/reference/rpc/v25/LabelService) | W `MutateLabels` |
| [LocalServicesLeadService](https://developers.google.com/google-ads/api/reference/rpc/v25/LocalServicesLeadService) | W `AppendLeadConversation`<br>W `ProvideLeadFeedback` |
| [MultiPartyAuthReviewService](https://developers.google.com/google-ads/api/reference/rpc/v25/MultiPartyAuthReviewService) | W `ResolveMultiPartyAuthReview` |
| [OfflineUserDataJobService](https://developers.google.com/google-ads/api/reference/rpc/v25/OfflineUserDataJobService) | W `CreateOfflineUserDataJob`<br>W `AddOfflineUserDataJobOperations`<br>W `RunOfflineUserDataJob` |
| [PaymentsAccountService](https://developers.google.com/google-ads/api/reference/rpc/v25/PaymentsAccountService) | R `ListPaymentsAccounts` |
| [ProductLinkInvitationService](https://developers.google.com/google-ads/api/reference/rpc/v25/ProductLinkInvitationService) | W `CreateProductLinkInvitation`<br>W `UpdateProductLinkInvitation`<br>W `RemoveProductLinkInvitation` |
| [ProductLinkService](https://developers.google.com/google-ads/api/reference/rpc/v25/ProductLinkService) | W `CreateProductLink`<br>W `RemoveProductLink` |
| [ReachPlanService](https://developers.google.com/google-ads/api/reference/rpc/v25/ReachPlanService) | R `ListPlannableLocations`<br>R `ListPlannableProducts`<br>R `ListPlannableUserLists`<br>R `ListPlannableUserInterests`<br>S `GenerateConversionRates`<br>S `GenerateReachForecast` |
| [RecommendationService](https://developers.google.com/google-ads/api/reference/rpc/v25/RecommendationService) | W `ApplyRecommendation`<br>W `DismissRecommendation`<br>S `GenerateRecommendations` |
| [RecommendationSubscriptionService](https://developers.google.com/google-ads/api/reference/rpc/v25/RecommendationSubscriptionService) | W `MutateRecommendationSubscription` |
| [RemarketingActionService](https://developers.google.com/google-ads/api/reference/rpc/v25/RemarketingActionService) | W `MutateRemarketingActions` |
| [ReservationService](https://developers.google.com/google-ads/api/reference/rpc/v25/ReservationService) | R `QuoteCampaigns`<br>W `BookCampaigns` |
| [ShareablePreviewService](https://developers.google.com/google-ads/api/reference/rpc/v25/ShareablePreviewService) | S `GenerateShareablePreviews` |
| [SharedCriterionService](https://developers.google.com/google-ads/api/reference/rpc/v25/SharedCriterionService) | W `MutateSharedCriteria` |
| [SharedSetService](https://developers.google.com/google-ads/api/reference/rpc/v25/SharedSetService) | W `MutateSharedSets` |
| [SmartCampaignSettingService](https://developers.google.com/google-ads/api/reference/rpc/v25/SmartCampaignSettingService) | R `GetSmartCampaignStatus`<br>W `MutateSmartCampaignSettings` |
| [SmartCampaignSuggestService](https://developers.google.com/google-ads/api/reference/rpc/v25/SmartCampaignSuggestService) | S `SuggestSmartCampaignBudgetOptions`<br>S `SuggestSmartCampaignAd`<br>S `SuggestKeywordThemes` |
| [ThirdPartyAppAnalyticsLinkService](https://developers.google.com/google-ads/api/reference/rpc/v25/ThirdPartyAppAnalyticsLinkService) | W `RegenerateShareableLinkId` |
| [TravelAssetSuggestionService](https://developers.google.com/google-ads/api/reference/rpc/v25/TravelAssetSuggestionService) | S `SuggestTravelAssets` |
| [UserDataService](https://developers.google.com/google-ads/api/reference/rpc/v25/UserDataService) | W `UploadUserData` |
| [UserListCustomerTypeService](https://developers.google.com/google-ads/api/reference/rpc/v25/UserListCustomerTypeService) | W `MutateUserListCustomerTypes` |
| [UserListService](https://developers.google.com/google-ads/api/reference/rpc/v25/UserListService) | W `MutateUserLists` |
| [YouTubeVideoUploadService](https://developers.google.com/google-ads/api/reference/rpc/v25/YouTubeVideoUploadService) | W `CreateYouTubeVideoUpload`<br>W `UpdateYouTubeVideoUpload`<br>W `RemoveYouTubeVideoUpload` |

No active v25 service RPC was omitted from this inventory. The generic Google Ads mutation escape hatch covers the 64 variants in `MutateOperation`; specialized RPCs still need a typed action or their documented REST method.
