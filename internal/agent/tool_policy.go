package agent

type toolApprovalPolicy int

const (
	toolAutoRun toolApprovalPolicy = iota
	toolRequiresApproval
)

var staticToolApprovalPolicies = map[string]toolApprovalPolicy{
	"apply_patch":  toolRequiresApproval,
	"list_files":   toolAutoRun,
	"read_file":    toolAutoRun,
	"search_files": toolAutoRun,
	"use_skill":    toolAutoRun,
}

func staticToolPolicy(name string) (toolApprovalPolicy, bool) {
	policy, ok := staticToolApprovalPolicies[name]
	return policy, ok
}
