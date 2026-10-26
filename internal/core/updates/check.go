// Package updates checks CLI versions against local package manager state.
// It is read-only: no package is installed, updated or removed.
package updates

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"larder/internal/core/model"
	"larder/internal/platform"
)

type Options struct {
	CLIs    []model.CLIInfo
	Skills  []model.Skill
	Dirs    []string
	Runner  platform.CommandRunner
	FS      platform.FileSystem
	Fetcher platform.Fetcher
	Timeout time.Duration
	// NpmCacheDir keeps npm's read-only metadata query out of a possibly
	// root-owned global cache. It is app-owned scratch space, never user assets.
	NpmCacheDir string
}

type brewFormula struct {
	Name              string   `json:"name"`
	InstalledVersions []string `json:"installed_versions"`
	CurrentVersion    string   `json:"current_version"`
}

type npmPackage struct {
	Current string `json:"current"`
	Wanted  string `json:"wanted"`
	Latest  string `json:"latest"`
}

func Check(ctx context.Context, opts Options) model.UpdateReport {
	report := model.UpdateReport{CheckedAt: time.Now(), Scanned: len(opts.CLIs)}
	if opts.Runner == nil {
		opts.Runner = platform.OSCommandRunner{}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 60 * time.Second
	}
	runner := opts.Runner
	type group struct {
		item model.UpdateItem
	}
	groups := map[string]*group{}
	var order []string
	owners := map[string]bool{}
	for _, cli := range opts.CLIs {
		if cli.Owner != "Homebrew" && cli.Owner != "npm" && cli.Owner != "winget" {
			continue
		}
		owners[cli.Owner] = true
		key := cli.Owner + "\x00" + cli.OwnerDetail
		if cli.OwnerDetail == "" {
			key = cli.ID
		}
		current, ok := groups[key]
		if !ok {
			name := cli.OwnerDetail
			if name == "" {
				name = cli.Name
			}
			current = &group{item: model.UpdateItem{
				Kind: "cli", CLIID: cli.ID, Owner: cli.Owner, Package: cli.OwnerDetail, Name: name,
				CurrentVersion: cli.Version, State: "unknown", Message: "尚未检查",
			}}
			if cli.OwnerDetail == "" {
				current.item.Message = "没有可用的包名"
			}
			groups[key] = current
			order = append(order, key)
		}
		if cli.Version != "" && current.item.CurrentVersion == "" {
			current.item.CurrentVersion = cli.Version
		}
		current.item.Commands = append(current.item.Commands, cli.Name)
	}
	for _, key := range order {
		report.Items = append(report.Items, groups[key].item)
	}
	if len(report.Items) > 0 {
		brewOutdated := map[string]string{}
		npmOutdated := map[string]string{}
		wingetOutdated := map[string]string{}
		brewChecked, npmChecked, wingetChecked := false, false, false

		if owners["Homebrew"] {
			if brew, ok := runner.Find("brew", opts.Dirs); ok {
				runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
				stdout, stderr, err := runner.Run(runCtx, brew, []string{"outdated", "--json=v2"}, []string{"HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_INSTALL_CLEANUP=1"})
				cancel()
				var data struct {
					Formulae []brewFormula `json:"formulae"`
				}
				if json.Unmarshal(stdout, &data) == nil {
					brewChecked = true
					for _, formula := range data.Formulae {
						brewOutdated[formula.Name] = formula.CurrentVersion
					}
				} else {
					report.Errors = append(report.Errors, fmt.Sprintf("brew outdated 失败: %v %s", err, strings.TrimSpace(string(stderr))))
				}
			} else {
				report.Errors = append(report.Errors, "未找到 brew，无法检查 Homebrew CLI")
			}
		}

		if owners["npm"] {
			if npm, ok := runner.Find("npm", opts.Dirs); ok {
				runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
				npmEnv := []string{"NPM_CONFIG_UPDATE_NOTIFIER=false"}
				if opts.NpmCacheDir != "" {
					npmEnv = append(npmEnv, "NPM_CONFIG_CACHE="+opts.NpmCacheDir)
				}
				stdout, stderr, err := runner.Run(runCtx, npm, []string{"outdated", "-g", "--json"}, npmEnv)
				cancel()
				parsed := map[string]npmPackage{}
				if json.Unmarshal(stdout, &parsed) == nil {
					npmChecked = true
					for name, pkg := range parsed {
						npmOutdated[name] = pkg.Latest
					}
				} else {
					report.Errors = append(report.Errors, fmt.Sprintf("npm outdated 失败: %v %s", err, strings.TrimSpace(string(stderr))))
				}
			} else {
				report.Errors = append(report.Errors, "未找到 npm，无法检查 npm CLI")
			}
		}

		if owners["winget"] {
			if winget, ok := runner.Find("winget", opts.Dirs); ok {
				runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
				stdout, stderr, err := runner.Run(runCtx, winget, []string{"upgrade", "--include-unknown", "--accept-source-agreements", "--disable-interactivity"}, nil)
				cancel()
				parsed := parseWingetUpgrades(string(stdout))
				if len(parsed) > 0 || err == nil {
					wingetChecked = true
					for name, latest := range parsed {
						wingetOutdated[strings.ToLower(name)] = latest
					}
				} else {
					report.Errors = append(report.Errors, fmt.Sprintf("winget upgrade 失败: %v %s", err, strings.TrimSpace(string(stderr))))
				}
			} else {
				report.Errors = append(report.Errors, "未找到 winget，无法检查 winget CLI")
			}
		}

		for i := range report.Items {
			item := &report.Items[i]
			var latest string
			var checked bool
			switch item.Owner {
			case "Homebrew":
				latest, checked = brewOutdated[item.Package], brewChecked
			case "npm":
				latest, checked = npmOutdated[item.Package], npmChecked
			case "winget":
				latest, checked = wingetOutdated[strings.ToLower(item.Package)], wingetChecked
			}
			if !checked {
				item.State = "error"
				item.Message = "包管理器不可用，未能检查"
				continue
			}
			if latest != "" {
				item.State = "available"
				item.LatestVersion = latest
				item.Message = "有新版本"
				report.Available++
				continue
			}
			if item.Package == "" {
				item.State = "unknown"
				item.Message = "无法确定包名"
				continue
			}
			item.State = "current"
			item.Message = "包管理器未报告新版本"
			report.Current++
		}
	}

	checkSkills(ctx, opts, &report)
	report.CheckedAt = time.Now()
	return report
}

var wingetColumns = regexp.MustCompile(`\s{2,}`)

// parseWingetUpgrades reads the table emitted by
// `winget upgrade --include-unknown`. Winget has no JSON output for this view.
func parseWingetUpgrades(output string) map[string]string {
	result := map[string]string{}
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "Name ") {
			continue
		}
		fields := wingetColumns.Split(line, -1)
		if len(fields) < 5 {
			continue
		}
		id := strings.TrimSpace(fields[1])
		available := strings.TrimSpace(fields[3])
		if id != "" && available != "" && strings.Contains(id, ".") {
			result[id] = available
		}
	}
	return result
}
