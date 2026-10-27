package clis

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"

	"larder/internal/platform"
)

// metadataResolver derives versions from package-manager metadata on disk.
// It deliberately never executes a command, so the scan stays read-only and
// safe for unknown binaries.
type metadataResolver struct {
	fs      platform.FileSystem
	home    string
	conda   map[string]map[string]condaBin
	cargo   map[string]cargoBin
	cargoOK bool
}

type condaBin struct {
	version string
	name    string
}

type cargoBin struct {
	version string
	name    string
}

func newMetadataResolver(fs platform.FileSystem, home string) *metadataResolver {
	return &metadataResolver{fs: fs, home: home, conda: map[string]map[string]condaBin{}}
}

func (r *metadataResolver) version(name, path, target, owner string) (string, string) {
	switch owner {
	case "Homebrew":
		return homebrewVersion(target)
	case "npm":
		return r.npmVersion(target)
	case "Conda":
		return r.condaVersion(path)
	case "Cargo":
		return r.cargoVersion(name)
	case "winget":
		return wingetVersion(target)
	default:
		return standaloneVersion(target)
	}
}

var standaloneVersionPattern = regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+)(?:-|$)`)

// standaloneVersion reads versions from native release layouts such as
// ~/.codex/packages/standalone/releases/<version>-<arch>/bin/codex.
func standaloneVersion(target string) (string, string) {
	if resolved, err := filepath.EvalSymlinks(target); err == nil {
		target = resolved
	}
	slash := filepath.ToSlash(target)
	if !strings.Contains(slash, "/standalone/") && !strings.Contains(slash, "/releases/") && !strings.Contains(slash, "/versions/") {
		return "", ""
	}
	parts := strings.Split(slash, "/")
	for index := len(parts) - 2; index >= 0; index-- {
		if match := standaloneVersionPattern.FindStringSubmatch(parts[index]); len(match) == 2 {
			return match[1], "standalone"
		}
	}
	return "", ""
}

func homebrewVersion(target string) (string, string) {
	slash := filepath.ToSlash(target)
	idx := strings.Index(slash, "/Cellar/")
	if idx < 0 {
		return "", ""
	}
	parts := strings.Split(slash[idx+len("/Cellar/"):], "/")
	if len(parts) < 2 {
		return "", ""
	}
	return parts[1], parts[0]
}

func wingetVersion(target string) (string, string) {
	slash := filepath.ToSlash(target)
	idx := strings.Index(strings.ToLower(slash), "/winget/packages/")
	if idx < 0 {
		return "", ""
	}
	rest := strings.Split(slash[idx+len("/winget/packages/"):], "/")
	if len(rest) == 0 {
		return "", ""
	}
	folder := rest[0]
	cut := strings.LastIndex(folder, "_")
	if cut <= 0 || cut == len(folder)-1 {
		return "", folder
	}
	return folder[cut+1:], folder[:cut]
}

func (r *metadataResolver) npmVersion(target string) (string, string) {
	slash := filepath.ToSlash(target)
	idx := strings.Index(slash, "/node_modules/")
	if idx < 0 {
		return "", ""
	}
	rest := strings.Split(slash[idx+len("/node_modules/"):], "/")
	if len(rest) == 0 {
		return "", ""
	}
	pkg := rest[0]
	if strings.HasPrefix(pkg, "@") && len(rest) > 1 {
		pkg += "/" + rest[1]
	}
	root := filepath.FromSlash(slash[:idx] + "/node_modules/" + pkg)
	raw, err := r.fs.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return "", ""
	}
	var pkgJSON struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(raw, &pkgJSON) != nil {
		return "", ""
	}
	return pkgJSON.Version, pkg
}

func (r *metadataResolver) condaVersion(path string) (string, string) {
	root := filepath.Dir(filepath.Dir(path))
	versions, ok := r.conda[root]
	if !ok {
		versions = map[string]condaBin{}
		entries, err := r.fs.ReadDir(filepath.Join(root, "conda-meta"))
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
					continue
				}
				raw, err := r.fs.ReadFile(filepath.Join(root, "conda-meta", entry.Name()))
				if err != nil {
					continue
				}
				var meta struct {
					Name    string   `json:"name"`
					Version string   `json:"version"`
					Files   []string `json:"files"`
				}
				if json.Unmarshal(raw, &meta) != nil {
					continue
				}
				for _, file := range meta.Files {
					if strings.HasPrefix(file, "bin/") {
						versions[filepath.Base(file)] = condaBin{version: meta.Version, name: meta.Name}
					}
				}
			}
		}
		r.conda[root] = versions
	}
	if value, ok := versions[filepath.Base(path)]; ok {
		return value.version, value.name
	}
	return "", ""
}

func (r *metadataResolver) cargoVersion(name string) (string, string) {
	if !r.cargoOK {
		r.cargoOK = true
		r.cargo = map[string]cargoBin{}
		if r.home != "" {
			raw, err := r.fs.ReadFile(filepath.Join(r.home, ".cargo", ".crates2.json"))
			if err == nil {
				var data struct {
					Installs map[string]struct {
						Bins []string `json:"bins"`
					} `json:"installs"`
				}
				if json.Unmarshal(raw, &data) == nil {
					for key, install := range data.Installs {
						fields := strings.Fields(key)
						if len(fields) < 2 {
							continue
						}
						for _, bin := range install.Bins {
							r.cargo[strings.TrimSuffix(bin, ".exe")] = cargoBin{version: fields[1], name: fields[0]}
						}
					}
				}
			}
		}
	}
	if value, ok := r.cargo[name]; ok {
		return value.version, value.name
	}
	return "", ""
}
