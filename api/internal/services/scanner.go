package services

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agentops-workspace/api/internal/domain"
)

var ignoredDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true, "coverage": true,
	"tmp": true, ".cache": true, ".next": true, ".nuxt": true, ".dart_tool": true, ".idea": true, ".vscode": true,
}

var managedFiles = map[string]bool{
	"AGENTS.md":                  true,
	"README.md":                  true,
	".agentops/project.yaml":     true,
	".agentops/assets.lock.yaml": true,
}

type RepoScanner struct{}

const (
	maxScanDepth      = 12
	maxScanCandidates = 1000
	maxKeywordRead    = 64 * 1024
)

var agentKeywords = []string{
	"agent", "agents", "agentic", "codex", "claude", "assistant", "llm", "automation", "workflow", "prompt", "subagent", "tool", "mcp",
}

func (RepoScanner) Tree(repoPath string, maxDepth int, showManaged bool) (domain.RepoTreeNode, error) {
	if maxDepth <= 0 || maxDepth > 12 {
		maxDepth = 5
	}
	root := domain.RepoTreeNode{Name: filepath.Base(repoPath), Path: ".", Type: "directory"}
	children, err := scanChildren(repoPath, ".", 0, maxDepth, showManaged)
	if err != nil {
		return root, err
	}
	root.Children = children
	return root, nil
}

func (RepoScanner) Detect(repoPath string) (domain.RepoScanResult, error) {
	result := domain.RepoScanResult{
		RepoPath:    repoPath,
		GitRepo:     exists(filepath.Join(repoPath, ".git")),
		Readable:    isReadable(repoPath),
		Candidates:  []domain.RepoScanCandidate{},
		IgnoredDirs: []string{},
	}
	seenCandidates := map[string]bool{}
	seenIgnored := map[string]bool{}
	err := filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(repoPath, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if ignoredDirs[name] {
				if !seenIgnored[rel] {
					seenIgnored[rel] = true
					result.IgnoredDirs = append(result.IgnoredDirs, rel)
				}
				return filepath.SkipDir
			}
			if scanDepth(rel) > maxScanDepth {
				if !seenIgnored[rel] {
					seenIgnored[rel] = true
					result.IgnoredDirs = append(result.IgnoredDirs, rel)
				}
				return filepath.SkipDir
			}
			switch name {
			case "agents":
				addScanCandidate(&result, seenCandidates, domain.RepoScanCandidate{Path: rel, Kind: "agents_directory", Reason: "directory named agents"})
			case ".codex":
				addScanCandidate(&result, seenCandidates, domain.RepoScanCandidate{Path: rel, Kind: "dot_codex_directory", Reason: "directory named .codex"})
			}
			return nil
		}
		result.ScannedFiles++
		if len(result.Candidates) >= maxScanCandidates {
			result.Truncated = true
			return filepath.SkipAll
		}
		candidate, ok := classifyScanCandidate(repoPath, rel, d)
		if ok {
			addScanCandidate(&result, seenCandidates, candidate)
		}
		return nil
	})
	sort.Slice(result.Candidates, func(i, j int) bool {
		if result.Candidates[i].Kind != result.Candidates[j].Kind {
			return result.Candidates[i].Kind < result.Candidates[j].Kind
		}
		return result.Candidates[i].Path < result.Candidates[j].Path
	})
	sort.Strings(result.IgnoredDirs)
	return result, err
}

func isReadable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func scanChildren(root, rel string, depth, maxDepth int, showManaged bool) ([]domain.RepoTreeNode, error) {
	if depth >= maxDepth {
		return nil, nil
	}
	dir := filepath.Join(root, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var nodes []domain.RepoTreeNode
	for _, entry := range entries {
		name := entry.Name()
		childRel := name
		if rel != "." {
			childRel = filepath.ToSlash(filepath.Join(rel, name))
		}
		if entry.IsDir() {
			if ignoredDirs[name] {
				continue
			}
			children, err := scanChildren(root, childRel, depth+1, maxDepth, showManaged)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, domain.RepoTreeNode{Name: name, Path: childRel, Type: "directory", Children: children})
			continue
		}
		if showManaged && isManagedTreeFile(childRel) {
			nodes = append(nodes, domain.RepoTreeNode{Name: name, Path: childRel, Type: "managed_file"})
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == "directory"
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

func isManagedTreeFile(path string) bool {
	if managedFiles[path] || strings.HasPrefix(path, ".agentops/runs/") {
		return true
	}
	typ, _ := classifyManagedFile(path)
	return typ != ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func addScanCandidate(result *domain.RepoScanResult, seen map[string]bool, candidate domain.RepoScanCandidate) {
	key := candidate.Kind + ":" + candidate.Path
	if seen[key] || len(result.Candidates) >= maxScanCandidates {
		if len(result.Candidates) >= maxScanCandidates {
			result.Truncated = true
		}
		return
	}
	seen[key] = true
	result.Candidates = append(result.Candidates, candidate)
}

func classifyScanCandidate(repoPath, rel string, d os.DirEntry) (domain.RepoScanCandidate, bool) {
	name := d.Name()
	info, _ := d.Info()
	var size int64
	if info != nil {
		size = info.Size()
	}
	base := strings.ToLower(name)
	pathLower := strings.ToLower(rel)
	matches := matchedAgentKeywords(pathLower)
	isMarkdown := strings.HasSuffix(base, ".md")
	if isMarkdown {
		matches = mergeKeywords(matches, matchedAgentKeywords(readKeywordSample(filepath.Join(repoPath, filepath.FromSlash(rel)))))
	}
	candidate := domain.RepoScanCandidate{Path: rel, SizeBytes: size, MatchedKeywords: matches}
	switch {
	case name == "AGENTS.md":
		candidate.Kind = "agents_file"
		candidate.Reason = "well-known agent instruction file"
	case name == "DESIGN.md":
		candidate.Kind = "design_file"
		candidate.Reason = "well-known design document"
	case strings.HasPrefix(rel, "agents/"):
		candidate.Kind = "agents_directory_file"
		candidate.Reason = "file inside agents directory"
	case strings.HasPrefix(rel, ".codex/"):
		candidate.Kind = "dot_codex_directory_file"
		candidate.Reason = "file inside .codex directory"
	case isRunReportPath(rel):
		candidate.Kind = "run_report"
		candidate.Reason = "AgentOps run report"
	case isReadmeMarkdown(name) && len(matches) > 0:
		candidate.Kind = "agent_readme"
		candidate.Reason = "README matched agent-related keywords"
	case isMarkdown && len(matches) > 0:
		candidate.Kind = "agent_markdown"
		candidate.Reason = "Markdown matched agent-related keywords"
	case isAgentOpsGenerated(rel):
		candidate.Kind = "agentops_generated"
		candidate.Reason = "known AgentOps generated file"
	default:
		return domain.RepoScanCandidate{}, false
	}
	return candidate, true
}

func isRunReportPath(path string) bool {
	parts := strings.Split(path, "/")
	return len(parts) == 4 && parts[0] == ".agentops" && parts[1] == "runs" && parts[3] == "run.report.json"
}

func isReadmeMarkdown(name string) bool {
	upper := strings.ToUpper(name)
	return strings.HasPrefix(upper, "README") && strings.HasSuffix(upper, ".MD")
}

func isAgentOpsGenerated(path string) bool {
	if path == ".agentops/project.yaml" || path == ".agentops/assets.lock.yaml" {
		return true
	}
	if path == "README.md" {
		return false
	}
	typ, _ := classifyManagedFile(path)
	return typ != ""
}

func scanDepth(path string) int {
	if path == "" || path == "." {
		return 0
	}
	return len(strings.Split(path, "/"))
}

func readKeywordSample(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	limited, err := io.ReadAll(io.LimitReader(f, maxKeywordRead))
	if err != nil {
		return ""
	}
	return strings.ToLower(string(limited))
}

func matchedAgentKeywords(text string) []string {
	var out []string
	for _, keyword := range agentKeywords {
		if strings.Contains(text, keyword) {
			out = append(out, keyword)
		}
	}
	return out
}

func mergeKeywords(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range [][]string{a, b} {
		for _, keyword := range list {
			if !seen[keyword] {
				seen[keyword] = true
				out = append(out, keyword)
			}
		}
	}
	return out
}
