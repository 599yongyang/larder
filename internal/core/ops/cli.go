package ops

import (
	"fmt"

	"larder/internal/core/model"
)

// PlanCLICommand builds a package-manager command preview. It never runs the
// command; execution goes through the service after explicit confirmation.
func PlanCLICommand(cli model.CLIInfo, uninstall bool, latest string) (model.Plan, error) {
	if cli.OwnerDetail == "" {
		return model.Plan{}, fmt.Errorf("missing package name for %s", cli.Name)
	}
	plan := model.Plan{
		Operation:        "cli_update",
		Title:            "更新 CLI",
		TitleMessage:     planTitle("plan.updateCli.title", "更新 CLI"),
		SummaryMessage:   planMessage("plan.summary.cliUpdate", "使用 {owner} 更新 {name}（{detail}）", map[string]string{"owner": cli.Owner, "name": cli.Name, "detail": cli.OwnerDetail}),
		RequiresSnapshot: true,
		Executable:       true,
		Impact:           []string{},
	}
	if uninstall {
		plan.Operation = "cli_uninstall"
		plan.Title = "卸载 CLI"
		plan.Summary = fmt.Sprintf("使用 %s 卸载 %s（%s）", cli.Owner, cli.OwnerDetail, cli.Name)
	}
	switch cli.Owner {
	case "Homebrew":
		if uninstall {
			plan.Command, plan.Args = "brew", []string{"uninstall", "--formula", cli.OwnerDetail}
		} else {
			plan.Command, plan.Args = "brew", []string{"upgrade", "--formula", cli.OwnerDetail}
		}
	case "npm":
		if uninstall {
			plan.Command, plan.Args = "npm", []string{"uninstall", "-g", cli.OwnerDetail}
		} else {
			spec := cli.OwnerDetail
			if latest != "" {
				spec += "@" + latest
			} else {
				spec += "@latest"
			}
			plan.Command, plan.Args = "npm", []string{"install", "-g", spec}
		}
	case "Bun":
		if uninstall {
			plan.Command, plan.Args = "bun", []string{"remove", "-g", cli.OwnerDetail}
		} else {
			plan.Command, plan.Args = "bun", []string{"update", "-g", cli.OwnerDetail}
		}
	case "Cargo":
		if uninstall {
			plan.Command, plan.Args = "cargo", []string{"uninstall", cli.OwnerDetail}
		} else {
			plan.Command, plan.Args = "cargo", []string{"install", cli.OwnerDetail}
		}
	case "Conda":
		if uninstall {
			plan.Command, plan.Args = "conda", []string{"remove", "-y", cli.OwnerDetail}
		} else {
			plan.Command, plan.Args = "conda", []string{"update", "-y", cli.OwnerDetail}
		}
	case "pipx":
		if uninstall {
			plan.Command, plan.Args = "pipx", []string{"uninstall", cli.OwnerDetail}
		} else {
			plan.Command, plan.Args = "pipx", []string{"upgrade", cli.OwnerDetail}
		}
	case "uv":
		if uninstall {
			plan.Command, plan.Args = "uv", []string{"tool", "uninstall", cli.OwnerDetail}
		} else {
			plan.Command, plan.Args = "uv", []string{"tool", "upgrade", cli.OwnerDetail}
		}
	case "winget":
		if uninstall {
			plan.Command, plan.Args = "winget", []string{"uninstall", "--id", cli.OwnerDetail, "--exact", "--accept-source-agreements", "--disable-interactivity"}
		} else {
			plan.Command, plan.Args = "winget", []string{"upgrade", "--id", cli.OwnerDetail, "--exact", "--accept-source-agreements", "--accept-package-agreements", "--disable-interactivity"}
		}
	default:
		return model.Plan{}, fmt.Errorf("unsupported CLI owner: %s", cli.Owner)
	}
	addWarning(&plan, "plan.warning.packageManagerExec", "命令由包管理器执行；应用不会直接删除文件。", nil)
	if uninstall {
		addWarning(&plan, "plan.warning.uninstallAdmin", "卸载可能需要管理员权限；当前版本只记录命令与旧版本，不提供自动回滚。", nil)
	} else {
		addWarning(&plan, "plan.warning.updatePackageManager", "更新由包管理器决定；旧版本不一定能自动恢复。", nil)
	}
	return plan, nil
}
