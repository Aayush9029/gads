package commands

type OperationClass string

const (
	OperationRead    OperationClass = "R"
	OperationWrite   OperationClass = "W"
	OperationSuggest OperationClass = "S"
)

type ServiceOperation struct {
	Name  string
	Class OperationClass
}

type GoogleAdsService struct {
	Name       string
	Operations []ServiceOperation
}

type ServiceMatrixSource struct {
	Name string
	URL  string
}

type ServiceMatrixInfo struct {
	APIVersion string
	Released   string
	Sunset     string
	Verified   string
	Sources    []ServiceMatrixSource
}

var OfficialServiceMatrixInfo = ServiceMatrixInfo{
	APIVersion: "v25",
	Released:   "2026-07-22",
	Sunset:     "2027-08",
	Verified:   "2026-08-07",
	Sources: []ServiceMatrixSource{
		{Name: "v25 RPC reference", URL: "https://developers.google.com/google-ads/api/reference/rpc/v25/overview"},
		{Name: "v25 protocol definitions", URL: "https://github.com/googleapis/googleapis/tree/master/google/ads/googleads/v25/services"},
		{Name: "RPC to REST mapping", URL: "https://developers.google.com/google-ads/api/rest/common/others"},
		{Name: "release notes", URL: "https://developers.google.com/google-ads/api/docs/release-notes"},
		{Name: "sunset schedule", URL: "https://developers.google.com/google-ads/api/docs/sunset-dates"},
	},
}

func operation(class OperationClass, name string) ServiceOperation {
	return ServiceOperation{Name: name, Class: class}
}

var OfficialServiceMatrix = []GoogleAdsService{
	{Name: "AccountBudgetProposalService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAccountBudgetProposal")}},
	{Name: "AccountLinkService", Operations: []ServiceOperation{operation(OperationWrite, "CreateAccountLink"), operation(OperationWrite, "MutateAccountLink")}},
	{Name: "AdGroupAdLabelService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAdGroupAdLabels")}},
	{Name: "AdGroupAdService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAdGroupAds"), operation(OperationWrite, "RemoveAutomaticallyCreatedAssets")}},
	{Name: "AdGroupAssetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAdGroupAssets")}},
	{Name: "AdGroupAssetSetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAdGroupAssetSets")}},
	{Name: "AdGroupBidModifierService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAdGroupBidModifiers")}},
	{Name: "AdGroupCriterionCustomizerService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAdGroupCriterionCustomizers")}},
	{Name: "AdGroupCriterionLabelService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAdGroupCriterionLabels")}},
	{Name: "AdGroupCriterionService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAdGroupCriteria")}},
	{Name: "AdGroupCustomizerService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAdGroupCustomizers")}},
	{Name: "AdGroupLabelService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAdGroupLabels")}},
	{Name: "AdGroupService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAdGroups")}},
	{Name: "AdParameterService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAdParameters")}},
	{Name: "AdService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAds")}},
	{Name: "AssetGenerationService", Operations: []ServiceOperation{operation(OperationSuggest, "GenerateText"), operation(OperationSuggest, "GenerateImages")}},
	{Name: "AssetGroupAssetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAssetGroupAssets")}},
	{Name: "AssetGroupListingGroupFilterService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAssetGroupListingGroupFilters")}},
	{Name: "AssetGroupService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAssetGroups")}},
	{Name: "AssetGroupSignalService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAssetGroupSignals")}},
	{Name: "AssetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAssets")}},
	{Name: "AssetSetAssetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAssetSetAssets")}},
	{Name: "AssetSetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAssetSets")}},
	{Name: "AudienceInsightsService", Operations: []ServiceOperation{
		operation(OperationRead, "ListAudienceInsightsAttributes"), operation(OperationRead, "ListInsightsEligibleDates"),
		operation(OperationSuggest, "GenerateInsightsFinderReport"), operation(OperationSuggest, "GenerateAudienceCompositionInsights"),
		operation(OperationSuggest, "GenerateAudienceDefinition"), operation(OperationSuggest, "GenerateSuggestedTargetingInsights"),
		operation(OperationSuggest, "GenerateAudienceOverlapInsights"), operation(OperationSuggest, "GenerateTargetingSuggestionMetrics"),
	}},
	{Name: "AudienceService", Operations: []ServiceOperation{operation(OperationWrite, "MutateAudiences")}},
	{Name: "AutomaticallyCreatedAssetRemovalService", Operations: []ServiceOperation{operation(OperationWrite, "RemoveCampaignAutomaticallyCreatedAsset")}},
	{Name: "BatchJobService", Operations: []ServiceOperation{
		operation(OperationRead, "ListBatchJobResults"), operation(OperationWrite, "MutateBatchJob"),
		operation(OperationWrite, "RunBatchJob"), operation(OperationWrite, "AddBatchJobOperations"),
	}},
	{Name: "BenchmarksService", Operations: []ServiceOperation{
		operation(OperationRead, "ListBenchmarksAvailableDates"), operation(OperationRead, "ListBenchmarksLocations"),
		operation(OperationRead, "ListBenchmarksProducts"), operation(OperationRead, "ListBenchmarksSources"),
		operation(OperationSuggest, "GenerateBenchmarksMetrics"),
	}},
	{Name: "BiddingDataExclusionService", Operations: []ServiceOperation{operation(OperationWrite, "MutateBiddingDataExclusions")}},
	{Name: "BiddingSeasonalityAdjustmentService", Operations: []ServiceOperation{operation(OperationWrite, "MutateBiddingSeasonalityAdjustments")}},
	{Name: "BiddingStrategyService", Operations: []ServiceOperation{operation(OperationWrite, "MutateBiddingStrategies")}},
	{Name: "BillingSetupService", Operations: []ServiceOperation{operation(OperationWrite, "MutateBillingSetup")}},
	{Name: "BrandSuggestionService", Operations: []ServiceOperation{operation(OperationSuggest, "SuggestBrands")}},
	{Name: "CampaignAssetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCampaignAssets")}},
	{Name: "CampaignAssetSetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCampaignAssetSets")}},
	{Name: "CampaignBidModifierService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCampaignBidModifiers")}},
	{Name: "CampaignBudgetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCampaignBudgets")}},
	{Name: "CampaignConversionGoalService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCampaignConversionGoals")}},
	{Name: "CampaignCriterionService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCampaignCriteria")}},
	{Name: "CampaignCustomizerService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCampaignCustomizers")}},
	{Name: "CampaignDraftService", Operations: []ServiceOperation{
		operation(OperationRead, "ListCampaignDraftAsyncErrors"), operation(OperationWrite, "MutateCampaignDrafts"), operation(OperationWrite, "PromoteCampaignDraft"),
	}},
	{Name: "CampaignGoalConfigService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCampaignGoalConfigs")}},
	{Name: "CampaignGroupService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCampaignGroups")}},
	{Name: "CampaignLabelService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCampaignLabels")}},
	{Name: "CampaignService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCampaigns"), operation(OperationWrite, "EnablePMaxBrandGuidelines")}},
	{Name: "CampaignSharedSetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCampaignSharedSets")}},
	{Name: "ContentCreatorInsightsService", Operations: []ServiceOperation{operation(OperationSuggest, "GenerateCreatorInsights"), operation(OperationSuggest, "GenerateTrendingInsights")}},
	{Name: "ConversionActionService", Operations: []ServiceOperation{operation(OperationWrite, "MutateConversionActions")}},
	{Name: "ConversionAdjustmentUploadService", Operations: []ServiceOperation{operation(OperationWrite, "UploadConversionAdjustments")}},
	{Name: "ConversionCustomVariableService", Operations: []ServiceOperation{operation(OperationWrite, "MutateConversionCustomVariables")}},
	{Name: "ConversionGoalCampaignConfigService", Operations: []ServiceOperation{operation(OperationWrite, "MutateConversionGoalCampaignConfigs")}},
	{Name: "ConversionUploadService", Operations: []ServiceOperation{operation(OperationWrite, "UploadClickConversions"), operation(OperationWrite, "UploadCallConversions")}},
	{Name: "ConversionValueRuleService", Operations: []ServiceOperation{operation(OperationWrite, "MutateConversionValueRules")}},
	{Name: "ConversionValueRuleSetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateConversionValueRuleSets")}},
	{Name: "CustomAudienceService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomAudiences")}},
	{Name: "CustomConversionGoalService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomConversionGoals")}},
	{Name: "CustomInterestService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomInterests")}},
	{Name: "CustomerAssetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomerAssets")}},
	{Name: "CustomerAssetSetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomerAssetSets")}},
	{Name: "CustomerClientLinkService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomerClientLink")}},
	{Name: "CustomerConversionGoalService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomerConversionGoals")}},
	{Name: "CustomerCustomizerService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomerCustomizers")}},
	{Name: "CustomerLabelService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomerLabels")}},
	{Name: "CustomerManagerLinkService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomerManagerLink"), operation(OperationWrite, "MoveManagerLink")}},
	{Name: "CustomerNegativeCriterionService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomerNegativeCriteria")}},
	{Name: "CustomerService", Operations: []ServiceOperation{
		operation(OperationRead, "ListAccessibleCustomers"), operation(OperationWrite, "MutateCustomer"), operation(OperationWrite, "CreateCustomerClient"),
	}},
	{Name: "CustomerSkAdNetworkConversionValueSchemaService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomerSkAdNetworkConversionValueSchema")}},
	{Name: "CustomerUserAccessInvitationService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomerUserAccessInvitation")}},
	{Name: "CustomerUserAccessService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomerUserAccess")}},
	{Name: "CustomizerAttributeService", Operations: []ServiceOperation{operation(OperationWrite, "MutateCustomizerAttributes")}},
	{Name: "DataLinkService", Operations: []ServiceOperation{operation(OperationWrite, "CreateDataLink"), operation(OperationWrite, "RemoveDataLink"), operation(OperationWrite, "UpdateDataLink")}},
	{Name: "ExperimentArmService", Operations: []ServiceOperation{operation(OperationWrite, "MutateExperimentArms")}},
	{Name: "ExperimentService", Operations: []ServiceOperation{
		operation(OperationRead, "ListExperimentAsyncErrors"), operation(OperationWrite, "MutateExperiments"), operation(OperationWrite, "EndExperiment"),
		operation(OperationWrite, "GraduateExperiment"), operation(OperationWrite, "ScheduleExperiment"), operation(OperationWrite, "PromoteExperiment"),
	}},
	{Name: "GeoTargetConstantService", Operations: []ServiceOperation{operation(OperationSuggest, "SuggestGeoTargetConstants")}},
	{Name: "GoalService", Operations: []ServiceOperation{operation(OperationWrite, "MutateGoals")}},
	{Name: "GoogleAdsFieldService", Operations: []ServiceOperation{operation(OperationRead, "GetGoogleAdsField"), operation(OperationRead, "SearchGoogleAdsFields")}},
	{Name: "GoogleAdsService", Operations: []ServiceOperation{operation(OperationRead, "Search"), operation(OperationRead, "SearchStream"), operation(OperationWrite, "Mutate")}},
	{Name: "IdentityVerificationService", Operations: []ServiceOperation{operation(OperationRead, "GetIdentityVerification"), operation(OperationWrite, "StartIdentityVerification")}},
	{Name: "IncentiveService", Operations: []ServiceOperation{operation(OperationRead, "FetchIncentive"), operation(OperationWrite, "ApplyIncentive")}},
	{Name: "InvoiceService", Operations: []ServiceOperation{operation(OperationRead, "ListInvoices")}},
	{Name: "KeywordPlanAdGroupKeywordService", Operations: []ServiceOperation{operation(OperationWrite, "MutateKeywordPlanAdGroupKeywords")}},
	{Name: "KeywordPlanAdGroupService", Operations: []ServiceOperation{operation(OperationWrite, "MutateKeywordPlanAdGroups")}},
	{Name: "KeywordPlanCampaignKeywordService", Operations: []ServiceOperation{operation(OperationWrite, "MutateKeywordPlanCampaignKeywords")}},
	{Name: "KeywordPlanCampaignService", Operations: []ServiceOperation{operation(OperationWrite, "MutateKeywordPlanCampaigns")}},
	{Name: "KeywordPlanIdeaService", Operations: []ServiceOperation{
		operation(OperationSuggest, "GenerateKeywordIdeas"), operation(OperationSuggest, "GenerateKeywordHistoricalMetrics"),
		operation(OperationSuggest, "GenerateAdGroupThemes"), operation(OperationSuggest, "GenerateKeywordForecastMetrics"),
	}},
	{Name: "KeywordPlanService", Operations: []ServiceOperation{operation(OperationWrite, "MutateKeywordPlans")}},
	{Name: "KeywordThemeConstantService", Operations: []ServiceOperation{operation(OperationSuggest, "SuggestKeywordThemeConstants")}},
	{Name: "LabelService", Operations: []ServiceOperation{operation(OperationWrite, "MutateLabels")}},
	{Name: "LocalServicesLeadService", Operations: []ServiceOperation{operation(OperationWrite, "AppendLeadConversation"), operation(OperationWrite, "ProvideLeadFeedback")}},
	{Name: "MultiPartyAuthReviewService", Operations: []ServiceOperation{operation(OperationWrite, "ResolveMultiPartyAuthReview")}},
	{Name: "OfflineUserDataJobService", Operations: []ServiceOperation{
		operation(OperationWrite, "CreateOfflineUserDataJob"), operation(OperationWrite, "AddOfflineUserDataJobOperations"), operation(OperationWrite, "RunOfflineUserDataJob"),
	}},
	{Name: "PaymentsAccountService", Operations: []ServiceOperation{operation(OperationRead, "ListPaymentsAccounts")}},
	{Name: "ProductLinkInvitationService", Operations: []ServiceOperation{
		operation(OperationWrite, "CreateProductLinkInvitation"), operation(OperationWrite, "UpdateProductLinkInvitation"), operation(OperationWrite, "RemoveProductLinkInvitation"),
	}},
	{Name: "ProductLinkService", Operations: []ServiceOperation{operation(OperationWrite, "CreateProductLink"), operation(OperationWrite, "RemoveProductLink")}},
	{Name: "ReachPlanService", Operations: []ServiceOperation{
		operation(OperationRead, "ListPlannableLocations"), operation(OperationRead, "ListPlannableProducts"),
		operation(OperationRead, "ListPlannableUserLists"), operation(OperationRead, "ListPlannableUserInterests"),
		operation(OperationSuggest, "GenerateConversionRates"), operation(OperationSuggest, "GenerateReachForecast"),
	}},
	{Name: "RecommendationService", Operations: []ServiceOperation{
		operation(OperationWrite, "ApplyRecommendation"), operation(OperationWrite, "DismissRecommendation"), operation(OperationSuggest, "GenerateRecommendations"),
	}},
	{Name: "RecommendationSubscriptionService", Operations: []ServiceOperation{operation(OperationWrite, "MutateRecommendationSubscription")}},
	{Name: "RemarketingActionService", Operations: []ServiceOperation{operation(OperationWrite, "MutateRemarketingActions")}},
	{Name: "ReservationService", Operations: []ServiceOperation{operation(OperationRead, "QuoteCampaigns"), operation(OperationWrite, "BookCampaigns")}},
	{Name: "ShareablePreviewService", Operations: []ServiceOperation{operation(OperationSuggest, "GenerateShareablePreviews")}},
	{Name: "SharedCriterionService", Operations: []ServiceOperation{operation(OperationWrite, "MutateSharedCriteria")}},
	{Name: "SharedSetService", Operations: []ServiceOperation{operation(OperationWrite, "MutateSharedSets")}},
	{Name: "SmartCampaignSettingService", Operations: []ServiceOperation{operation(OperationRead, "GetSmartCampaignStatus"), operation(OperationWrite, "MutateSmartCampaignSettings")}},
	{Name: "SmartCampaignSuggestService", Operations: []ServiceOperation{
		operation(OperationSuggest, "SuggestSmartCampaignBudgetOptions"), operation(OperationSuggest, "SuggestSmartCampaignAd"), operation(OperationSuggest, "SuggestKeywordThemes"),
	}},
	{Name: "ThirdPartyAppAnalyticsLinkService", Operations: []ServiceOperation{operation(OperationWrite, "RegenerateShareableLinkId")}},
	{Name: "TravelAssetSuggestionService", Operations: []ServiceOperation{operation(OperationSuggest, "SuggestTravelAssets")}},
	{Name: "UserDataService", Operations: []ServiceOperation{operation(OperationWrite, "UploadUserData")}},
	{Name: "UserListCustomerTypeService", Operations: []ServiceOperation{operation(OperationWrite, "MutateUserListCustomerTypes")}},
	{Name: "UserListService", Operations: []ServiceOperation{operation(OperationWrite, "MutateUserLists")}},
	{Name: "YouTubeVideoUploadService", Operations: []ServiceOperation{
		operation(OperationWrite, "CreateYouTubeVideoUpload"), operation(OperationWrite, "UpdateYouTubeVideoUpload"), operation(OperationWrite, "RemoveYouTubeVideoUpload"),
	}},
}

var NonMutateWriteOperations = []string{
	"AccountLinkService.CreateAccountLink",
	"AdGroupAdService.RemoveAutomaticallyCreatedAssets",
	"AutomaticallyCreatedAssetRemovalService.RemoveCampaignAutomaticallyCreatedAsset",
	"BatchJobService.RunBatchJob",
	"BatchJobService.AddBatchJobOperations",
	"CampaignDraftService.PromoteCampaignDraft",
	"CampaignService.EnablePMaxBrandGuidelines",
	"ConversionAdjustmentUploadService.UploadConversionAdjustments",
	"ConversionUploadService.UploadClickConversions",
	"ConversionUploadService.UploadCallConversions",
	"CustomerManagerLinkService.MoveManagerLink",
	"CustomerService.CreateCustomerClient",
	"DataLinkService.CreateDataLink",
	"DataLinkService.RemoveDataLink",
	"DataLinkService.UpdateDataLink",
	"ExperimentService.EndExperiment",
	"ExperimentService.GraduateExperiment",
	"ExperimentService.ScheduleExperiment",
	"ExperimentService.PromoteExperiment",
	"IdentityVerificationService.StartIdentityVerification",
	"IncentiveService.ApplyIncentive",
	"LocalServicesLeadService.AppendLeadConversation",
	"LocalServicesLeadService.ProvideLeadFeedback",
	"MultiPartyAuthReviewService.ResolveMultiPartyAuthReview",
	"OfflineUserDataJobService.CreateOfflineUserDataJob",
	"OfflineUserDataJobService.AddOfflineUserDataJobOperations",
	"OfflineUserDataJobService.RunOfflineUserDataJob",
	"ProductLinkInvitationService.CreateProductLinkInvitation",
	"ProductLinkInvitationService.UpdateProductLinkInvitation",
	"ProductLinkInvitationService.RemoveProductLinkInvitation",
	"ProductLinkService.CreateProductLink",
	"ProductLinkService.RemoveProductLink",
	"RecommendationService.ApplyRecommendation",
	"RecommendationService.DismissRecommendation",
	"ReservationService.BookCampaigns",
	"ThirdPartyAppAnalyticsLinkService.RegenerateShareableLinkId",
	"UserDataService.UploadUserData",
	"YouTubeVideoUploadService.CreateYouTubeVideoUpload",
	"YouTubeVideoUploadService.UpdateYouTubeVideoUpload",
	"YouTubeVideoUploadService.RemoveYouTubeVideoUpload",
}
