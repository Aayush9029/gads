package commands

import "strings"

const LocalOperation = "local"

func OperationForCommand(path string) string {
	switch path {
	case "auth login":
		return LocalOperation
	case "auth import", "auth status", "auth logout", "profile list", "profile show", "profile set", "profile use", "mutate plan":
		return LocalOperation
	case "accounts list":
		return "CustomerService.ListAccessibleCustomers"
	case "fields search", "fields show":
		return "GoogleAdsFieldService.SearchGoogleAdsFields"
	case "conversion upload-clicks":
		return "ConversionUploadService.UploadClickConversions"
	case "conversion upload-calls":
		return "ConversionUploadService.UploadCallConversions"
	case "conversion upload-adjustments":
		return "ConversionAdjustmentUploadService.UploadConversionAdjustments"
	case "billing accounts":
		return "PaymentsAccountService.ListPaymentsAccounts"
	case "billing invoices":
		return "InvoiceService.ListInvoices"
	case "billing setup-create", "billing setup-cancel":
		return "BillingSetupService.MutateBillingSetup"
	case "billing budget-proposal":
		return "AccountBudgetProposalService.MutateAccountBudgetProposal"
	case "query validate":
		return "GoogleAdsService.Search"
	case "mutate validate", "mutate execute":
		return "GoogleAdsService.Mutate"
	}
	group := strings.Split(path, " ")[0]
	switch group {
	case "accounts", "query", "report", "change-history", "constants":
		return "GoogleAdsService.Search"
	case "campaign", "budget", "ad-group", "ad", "asset", "conversion", "app-campaign":
		for _, command := range Registry {
			if command.Path == path && (command.Kind == "mutate" || command.Kind == "action") {
				return "GoogleAdsService.Mutate"
			}
		}
		return "GoogleAdsService.Search"
	case "billing":
		return "GoogleAdsService.Search"
	}
	return ""
}
