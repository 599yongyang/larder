package platform

import (
	"os"
	"path/filepath"
	"strings"
)

// ExecutableEnv returns base with PATH rebuilt from the executable directory,
// known user package-manager bin directories and the inherited PATH. Desktop
// launches often inherit only /usr/bin and friends, while node/npm/npx live in
// nvm, Homebrew, asdf or a user-local bin directory.
func ExecutableEnv(base []string, executable string) []string {
	dirs := []string{}
	if executable != "" {
		dirs = append(dirs, filepath.Dir(executable))
	}
	home, _ := os.UserHomeDir()
	dirs = append(dirs, LocalPaths{}.PathDirs(home)...)
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if strings.TrimSpace(dir) != "" {
			dirs = append(dirs, dir)
		}
	}
	seen := map[string]bool{}
	unique := dirs[:0]
	for _, dir := range dirs {
		dir = filepath.Clean(dir)
		if dir == "." || seen[dir] {
			continue
		}
		seen[dir] = true
		unique = append(unique, dir)
	}
	env := make([]string, 0, len(base)+1)
	for _, entry := range base {
		index := strings.IndexByte(entry, '=')
		if index >= 0 && strings.EqualFold(entry[:index], "PATH") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "PATH="+strings.Join(unique, string(os.PathListSeparator)))
}
