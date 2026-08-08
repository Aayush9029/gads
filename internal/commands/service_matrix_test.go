package commands

import (
	"strings"
	"testing"
)

func TestOfficialServiceMatrixInventory(t *testing.T) {
	if got, want := len(OfficialServiceMatrix), 110; got != want {
		t.Fatalf("service count = %d, want %d", got, want)
	}

	services := make(map[string]struct{}, len(OfficialServiceMatrix))
	operations := make(map[string]struct{}, 169)
	classCounts := map[OperationClass]int{}
	for _, service := range OfficialServiceMatrix {
		if service.Name == "" {
			t.Fatal("service has an empty name")
		}
		if _, exists := services[service.Name]; exists {
			t.Errorf("duplicate service %q", service.Name)
		}
		services[service.Name] = struct{}{}
		if len(service.Operations) == 0 {
			t.Errorf("service %q has no operations", service.Name)
		}
		for _, operation := range service.Operations {
			qualifiedName := service.Name + "." + operation.Name
			if _, exists := operations[qualifiedName]; exists {
				t.Errorf("duplicate operation %q", qualifiedName)
			}
			operations[qualifiedName] = struct{}{}
			switch operation.Class {
			case OperationRead, OperationWrite, OperationSuggest:
				classCounts[operation.Class]++
			default:
				t.Errorf("operation %q has invalid class %q", qualifiedName, operation.Class)
			}
		}
	}

	if got, want := len(operations), 169; got != want {
		t.Errorf("RPC count = %d, want %d", got, want)
	}
	for class, want := range map[OperationClass]int{
		OperationRead:    24,
		OperationWrite:   119,
		OperationSuggest: 26,
	} {
		if got := classCounts[class]; got != want {
			t.Errorf("class %s count = %d, want %d", class, got, want)
		}
	}
}

func TestNonMutateWritesAreExplicitlyInventoried(t *testing.T) {
	expected := []string{
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

	if got, want := len(NonMutateWriteOperations), len(expected); got != want {
		t.Fatalf("non-Mutate write count = %d, want %d", got, want)
	}
	inventory := make(map[string]OperationClass, 169)
	for _, service := range OfficialServiceMatrix {
		for _, operation := range service.Operations {
			inventory[service.Name+"."+operation.Name] = operation.Class
		}
	}
	explicit := make(map[string]struct{}, len(NonMutateWriteOperations))
	for _, qualifiedName := range NonMutateWriteOperations {
		if _, exists := explicit[qualifiedName]; exists {
			t.Errorf("duplicate explicit non-Mutate write %q", qualifiedName)
		}
		explicit[qualifiedName] = struct{}{}
		if strings.HasPrefix(qualifiedName[strings.LastIndex(qualifiedName, ".")+1:], "Mutate") {
			t.Errorf("explicit non-Mutate write %q begins with Mutate", qualifiedName)
		}
		if got := inventory[qualifiedName]; got != OperationWrite {
			t.Errorf("explicit non-Mutate operation %q has class %q, want %q", qualifiedName, got, OperationWrite)
		}
	}
	for _, qualifiedName := range expected {
		if _, exists := explicit[qualifiedName]; !exists {
			t.Errorf("missing explicit non-Mutate write %q", qualifiedName)
		}
	}
	for qualifiedName, class := range inventory {
		operationName := qualifiedName[strings.LastIndex(qualifiedName, ".")+1:]
		if class == OperationWrite && !strings.HasPrefix(operationName, "Mutate") {
			if _, exists := explicit[qualifiedName]; !exists {
				t.Errorf("write operation %q is not covered by explicit non-Mutate inventory", qualifiedName)
			}
		}
	}
}

func TestOfficialServiceMatrixSourceMetadata(t *testing.T) {
	if got, want := OfficialServiceMatrixInfo.APIVersion, "v25"; got != want {
		t.Errorf("API version = %q, want %q", got, want)
	}
	if OfficialServiceMatrixInfo.Verified == "" || len(OfficialServiceMatrixInfo.Sources) < 2 {
		t.Fatal("official service matrix must include verification and source metadata")
	}
	for _, source := range OfficialServiceMatrixInfo.Sources {
		if source.Name == "" || !strings.HasPrefix(source.URL, "https://") {
			t.Errorf("invalid source metadata: %#v", source)
		}
	}
}
