package agent

import "testing"

func TestStaticToolApprovalPolicies(t *testing.T) {
	expected := map[string]toolApprovalPolicy{
		"apply_patch":  toolRequiresApproval,
		"list_files":   toolAutoRun,
		"search_files": toolAutoRun,
		"use_skill":    toolAutoRun,
	}
	if len(staticToolApprovalPolicies) != len(expected) {
		t.Fatalf("unexpected static tool policy count: %+v", staticToolApprovalPolicies)
	}
	for name, want := range expected {
		got, ok := staticToolPolicy(name)
		if !ok {
			t.Fatalf("missing static tool policy for %s", name)
		}
		if got != want {
			t.Fatalf("unexpected static tool policy for %s: got %v want %v", name, got, want)
		}
	}
	if _, ok := staticToolPolicy("run_command"); ok {
		t.Fatal("run_command must use command classification instead of static tool policy")
	}
	if _, ok := staticToolPolicy("git_" + "status"); ok {
		t.Fatal("dedicated Git status tool must not be exposed as an agent tool")
	}
	if _, ok := staticToolPolicy("git_" + "diff"); ok {
		t.Fatal("dedicated Git diff tool must not be exposed as an agent tool")
	}
}
