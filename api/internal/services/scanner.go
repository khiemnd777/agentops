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
	"tmp": true, "temp": true, ".cache": true, ".next": true, ".nuxt": true, ".dart_tool": true, ".idea": true, ".vscode": true,
	".turbo": true, ".parcel-cache": true, ".pytest_cache": true, ".ruff_cache": true, ".mypy_cache": true, "__pycache__": true,
	"target": true, "out": true, ".expo": true, ".serverless": true, ".terraform": true, ".gradle": true, "Pods": true, "DerivedData": true,
	".venv": true, "venv": true, "env": true, ".bundle": true,
}

var ignoredFileNames = map[string]bool{
	".DS_Store": true, "Thumbs.db": true, "desktop.ini": true,
}

var ignoredFileExts = map[string]bool{
	".log": true, ".tmp": true, ".temp": true, ".bak": true, ".swp": true, ".swo": true, ".pyc": true, ".pyo": true,
}

var managedFiles = map[string]bool{
	"AGENTS.md":             true,
	"README.md":             true,
	".codex/project.json":   true,
	".codex/sync/lock.json": true,
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
	privatePath := filepath.Join(repoPath, ".codex")
	result := domain.RepoScanResult{
		RepoPath:    repoPath,
		PrivatePath: privatePath,
		GitRepo:     exists(filepath.Join(repoPath, ".git")),
		Readable:    isReadable(repoPath),
		Candidates:  []domain.RepoScanCandidate{},
		IgnoredDirs: []string{},
		Private:     scanSection(privatePath),
		Global:      scanGlobalCodexSection(),
	}
	if result.Global != nil {
		result.GlobalPath = result.Global.Path
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
			if shouldIgnoreDirPath(rel) {
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
		if shouldIgnoreFile(rel) {
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

func scanGlobalCodexSection() *domain.RepoScanSection {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return scanSection(filepath.Join(home, ".codex"))
}

func scanSection(path string) *domain.RepoScanSection {
	section := &domain.RepoScanSection{
		Path:        path,
		Exists:      exists(path),
		Readable:    isReadable(path),
		Candidates:  []domain.RepoScanCandidate{},
		IgnoredDirs: []string{},
	}
	if !section.Exists {
		return section
	}
	seenCandidates := map[string]bool{}
	seenIgnored := map[string]bool{}
	err := filepath.WalkDir(path, func(current string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(path, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if shouldIgnoreDirPath(rel) || scanDepth(rel) > maxScanDepth {
				if !seenIgnored[rel] {
					seenIgnored[rel] = true
					section.IgnoredDirs = append(section.IgnoredDirs, rel)
				}
				return filepath.SkipDir
			}
			return nil
		}
		if shouldIgnoreFile(rel) {
			return nil
		}
		section.ScannedFiles++
		if len(section.Candidates) >= maxScanCandidates {
			section.Truncated = true
			return filepath.SkipAll
		}
		candidate, ok := classifyScanCandidate(path, rel, d)
		if ok {
			key := candidate.Kind + ":" + candidate.Path
			if !seenCandidates[key] {
				seenCandidates[key] = true
				section.Candidates = append(section.Candidates, candidate)
			}
		}
		return nil
	})
	if err != nil {
		section.Readable = false
	}
	sort.Slice(section.Candidates, func(i, j int) bool {
		if section.Candidates[i].Kind != section.Candidates[j].Kind {
			return section.Candidates[i].Kind < section.Candidates[j].Kind
		}
		return section.Candidates[i].Path < section.Candidates[j].Path
	})
	sort.Strings(section.IgnoredDirs)
	return section
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
			if shouldIgnoreDirPath(childRel) {
				continue
			}
			children, err := scanChildren(root, childRel, depth+1, maxDepth, showManaged)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, domain.RepoTreeNode{Name: name, Path: childRel, Type: "directory", Children: children})
			continue
		}
		if shouldIgnoreFile(childRel) {
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
	if managedFiles[path] || strings.HasPrefix(path, ".codex/reports/runs/") {
		return true
	}
	typ, _ := classifyManagedFile(path)
	return typ != ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func shouldIgnoreDir(name string) bool {
	return ignoredDirs[name]
}

func shouldIgnoreDirPath(path string) bool {
	path = filepath.ToSlash(path)
	if path == ".agentops" || strings.HasPrefix(path, ".agentops/") {
		return true
	}
	if path == ".codex/scripts" || strings.HasPrefix(path, ".codex/scripts/") {
		return true
	}
	return shouldIgnoreDir(filepath.Base(path))
}

func shouldIgnoreFile(path string) bool {
	name := filepath.Base(filepath.ToSlash(path))
	if ignoredFileNames[name] {
		return true
	}
	lower := strings.ToLower(name)
	if lower == ".env" || strings.HasPrefix(lower, ".env.") {
		return true
	}
	if strings.HasSuffix(lower, "-debug.log") {
		return true
	}
	return ignoredFileExts[strings.ToLower(filepath.Ext(lower))]
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
	isYAML := strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml")
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
	case isRunReportPath(rel):
		candidate.Kind = "run_report"
		candidate.Reason = "Codex run report"
	case strings.HasPrefix(rel, ".codex/"):
		candidate.Kind = "dot_codex_directory_file"
		candidate.Reason = "file inside .codex directory"
	case isOpenAIConfigPath(rel) && isYAML:
		candidate.Kind = "openai_config"
		candidate.Reason = "OpenAI configuration file"
	case isReadmeMarkdown(name) && len(matches) > 0:
		candidate.Kind = "agent_readme"
		candidate.Reason = "README matched agent-related keywords"
	case isMarkdown:
		candidate.Kind = "agent_markdown"
		if len(matches) > 0 {
			candidate.Reason = "Markdown matched agent-related keywords"
		} else {
			candidate.Reason = "Markdown context file"
		}
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
	return len(parts) == 5 && parts[0] == ".codex" && parts[1] == "reports" && parts[2] == "runs" && parts[4] == "run.report.json"
}

func isReadmeMarkdown(name string) bool {
	upper := strings.ToUpper(name)
	return strings.HasPrefix(upper, "README") && strings.HasSuffix(upper, ".MD")
}

func isAgentOpsGenerated(path string) bool {
	if path == ".codex/project.json" || path == ".codex/sync/lock.json" {
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
