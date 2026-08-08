package commands

import (
	"strings"
	"testing"
)

func TestEveryCommandMapsToOfficialOperation(t *testing.T) {
	official := map[string]bool{}
	for _, service := range OfficialServiceMatrix {
		for _, operation := range service.Operations {
			official[service.Name+"."+operation.Name] = true
		}
	}
	for _, command := range Registry {
		operation := OperationForCommand(command.Path)
		if operation == "" {
			t.Errorf("command %q has no operation mapping", command.Path)
			continue
		}
		if operation != LocalOperation && !official[operation] {
			t.Errorf("command %q maps to missing official operation %q", command.Path, operation)
		}
	}
}

func TestEveryNonLocalCommandUsesExpectedSafetyClass(t *testing.T) {
	classes := map[string]OperationClass{}
	for _, service := range OfficialServiceMatrix {
		for _, operation := range service.Operations {
			classes[service.Name+"."+operation.Name] = operation.Class
		}
	}
	for _, command := range Registry {
		operation := OperationForCommand(command.Path)
		if operation == LocalOperation {
			continue
		}
		class := classes[operation]
		if command.Kind == "read" && class != OperationRead {
			t.Errorf("read command %q maps to %s operation %q", command.Path, class, operation)
		}
		if command.Kind == "validate" && strings.HasPrefix(command.Path, "query ") && class != OperationRead {
			t.Errorf("query validation %q must use a read operation", command.Path)
		}
		if (command.Kind == "mutate" || command.Kind == "action") && class != OperationWrite {
			t.Errorf("write command %q maps to %s operation %q", command.Path, class, operation)
		}
	}
}
