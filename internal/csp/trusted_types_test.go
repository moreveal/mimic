package csp

import "testing"

func TestTrustedTypesPolicyComposition(t *testing.T) {
	p := Parse("require-trusted-types-for 'script'; trusted-types a, trusted-types b; script-src 'none'").TrustedTypes()
	if !p.Required || !p.Enforced || len(p.Rules) != 2 || p.EvalBlocked != "script-src 'none'" {
		t.Fatalf("composed policy: %+v", p)
	}
	p = Parse("require-trusted-types-for 'SCRIPT'; require-trusted-types-for 'script'").TrustedTypes()
	if p.Required {
		t.Fatal("invalid first directive must not use the second directive")
	}
	set := ParseReportOnly("require-trusted-types-for 'script'; trusted-types 'none'; script-src 'none'; form-action 'none'")
	p = set.TrustedTypes()
	if !p.Required || p.Enforced || len(p.Rules) != 0 || p.EvalBlocked != "" {
		t.Fatalf("report only: %+v", p)
	}
	if !set.AllowsEventHandler("x") || !set.AllowsFormAction(nil, nil) {
		t.Fatal("report only must not enforce")
	}
	if allowed, _ := set.AllowsScript(nil, nil, true, false, ""); !allowed {
		t.Fatal("report only script blocked")
	}
}
