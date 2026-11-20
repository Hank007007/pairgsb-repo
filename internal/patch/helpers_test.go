package patch

import "patchsieve/internal/policy"

func admin() policy.Principal {
	return policy.Principal{Role: "admin", TenantID: "t1"}
}

func editor() policy.Principal {
	return policy.Principal{Role: "editor", TenantID: "t1"}
}
