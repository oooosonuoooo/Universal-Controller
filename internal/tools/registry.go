package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Tool struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Command     string `json:"command"`
	Installed   bool   `json:"installed"`
}

func CuratedCatalog() []Tool {
	catalog := []Tool{
		{Name: "adb", Category: "mobile", Description: "Android Debug Bridge", Command: "adb"},
		{Name: "ansible", Category: "devops", Description: "Automation and remote orchestration", Command: "ansible"},
		{Name: "docker", Category: "devops", Description: "Container lifecycle and image workflows", Command: "docker"},
		{Name: "ffmpeg", Category: "media", Description: "Media processing and transcoding", Command: "ffmpeg"},
		{Name: "git", Category: "developer", Description: "Version control", Command: "git"},
		{Name: "jq", Category: "developer", Description: "JSON processor", Command: "jq"},
		{Name: "kubectl", Category: "devops", Description: "Kubernetes control", Command: "kubectl"},
		{Name: "nmap", Category: "network", Description: "Network scanning and service enumeration", Command: "nmap"},
		{Name: "pwsh", Category: "shell", Description: "PowerShell Core", Command: "pwsh"},
		{Name: "python3", Category: "developer", Description: "Python runtime", Command: "python3"},
		{Name: "ssh", Category: "remote", Description: "Secure shell", Command: "ssh"},
		{Name: "terraform", Category: "devops", Description: "Infrastructure as code", Command: "terraform"},
		{Name: "tmux", Category: "shell", Description: "Terminal multiplexer", Command: "tmux"},
		{Name: "uv", Category: "developer", Description: "Python package and environment manager", Command: "uv"},
		{Name: "yt-dlp", Category: "media", Description: "Media retrieval utility", Command: "yt-dlp"},
	}
	for i := range catalog {
		_, err := exec.LookPath(catalog[i].Command)
		catalog[i].Installed = err == nil
	}
	sort.SliceStable(catalog, func(i, j int) bool {
		return catalog[i].Name < catalog[j].Name
	})
	return catalog
}

func DiscoverPathTools(limit int) []Tool {
	seen := map[string]struct{}{}
	entries := []Tool{}
	pathDirs := filepath.SplitList(os.Getenv("PATH"))
	for _, dir := range pathDirs {
		if dir == "" {
			continue
		}
		items, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, item := range items {
			if item.IsDir() {
				continue
			}
			name := item.Name()
			if runtime.GOOS == "windows" {
				name = strings.TrimSuffix(name, filepath.Ext(name))
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			entries = append(entries, Tool{
				Name:        name,
				Category:    "discovered",
				Description: "Discovered from PATH",
				Command:     name,
				Installed:   true,
			})
			if limit > 0 && len(entries) >= limit {
				sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
				return entries
			}
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

func FindByName(name string) (Tool, bool) {
	name = strings.TrimSpace(strings.ToLower(name))
	for _, tool := range CuratedCatalog() {
		if tool.Name == name || tool.Command == name {
			return tool, true
		}
	}
	for _, tool := range DiscoverPathTools(0) {
		if strings.ToLower(tool.Name) == name {
			return tool, true
		}
	}
	return Tool{}, false
}

func Render(tools []Tool) string {
	lines := make([]string, 0, len(tools))
	for _, tool := range tools {
		state := "missing"
		if tool.Installed {
			state = "installed"
		}
		lines = append(lines, fmt.Sprintf("%-16s %-12s %-10s %s", tool.Name, tool.Category, state, tool.Description))
	}
	return strings.Join(lines, "\n")
}
