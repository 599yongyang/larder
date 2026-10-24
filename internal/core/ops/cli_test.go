package ops_test

import (
	"reflect"
	"testing"

	"larder/internal/core/model"
	"larder/internal/core/ops"
)

func TestPlanCLICommandsAreFixedArgv(t *testing.T) {
	cases := []struct {
		owner     string
		uninstall bool
		want      []string
	}{
		{"Homebrew", false, []string{"upgrade", "--formula", "jq"}},
		{"Homebrew", true, []string{"uninstall", "--formula", "jq"}},
		{"npm", false, []string{"install", "-g", "foo@latest"}},
		{"npm", true, []string{"uninstall", "-g", "foo"}},
		{"winget", false, []string{"upgrade", "--id", "Owner.Tool", "--exact", "--accept-source-agreements", "--accept-package-agreements", "--disable-interactivity"}},
	}
	for _, tc := range cases {
		plan, err := ops.PlanCLICommand(model.CLIInfo{ID: "cli:x", Name: "x", Owner: tc.owner, OwnerDetail: "jq"}, tc.uninstall, "")
		if tc.owner == "npm" {
			plan, err = ops.PlanCLICommand(model.CLIInfo{ID: "cli:x", Name: "x", Owner: tc.owner, OwnerDetail: "foo"}, tc.uninstall, "")
		}
		if tc.owner == "winget" {
			plan, err = ops.PlanCLICommand(model.CLIInfo{ID: "cli:x", Name: "x", Owner: tc.owner, OwnerDetail: "Owner.Tool"}, tc.uninstall, "")
		}
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(plan.Args, tc.want) {
			t.Errorf("%s uninstall=%v args = %#v, want %#v", tc.owner, tc.uninstall, plan.Args, tc.want)
		}
	}
}
