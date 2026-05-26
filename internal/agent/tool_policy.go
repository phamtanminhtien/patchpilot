package agent

type toolApprovalPolicy int

const (
	toolAutoRun toolApprovalPolicy = iota
	toolRequiresApproval
)

var staticToolApprovalPolicies = map[string]toolApprovalPolicy{
	"apply_patch":  toolRequiresApproval,
	"list_files":   toolAutoRun,
	"search_files": toolAutoRun,
}

func staticToolPolicy(name string) (toolApprovalPolicy, bool) {
	policy, ok := staticToolApprovalPolicies[name]
	return policy, ok
}
