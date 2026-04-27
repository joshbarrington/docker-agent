package skills

import (
	"cmp"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/docker/docker-agent/pkg/paths"
)

const skillFile = "SKILL.md"

// Skill represents a loaded skill with its metadata and content location.
type Skill struct {
	Name          string
	Description   string
	FilePath      string
	BaseDir       string
	Files         []string
	Local         bool // true for filesystem-loaded skills, false for remote
	License       string
	Compatibility string
	Metadata      map[string]string
	AllowedTools  []string
	Context       string // "fork" to run the skill as an isolated sub-agent
	// Model is an optional model override applied while the skill runs as
	// a sub-agent (context: fork). It accepts either a named model from the
	// agent config or an inline "provider/model" reference (e.g.
	// "openai/gpt-4o-mini"). It is ignored for non-fork skills.
	Model string
}

// IsFork returns true when the skill should be executed in an isolated
// sub-agent context rather than inline in the current conversation.
// This matches Claude Code's `context: fork` frontmatter syntax.
func (s *Skill) IsFork() bool {
	return s.Context == "fork"
}

// Load discovers and loads skills from the given sources.
// Each source is either "local" (for filesystem-based skills) or an HTTP/HTTPS URL
// (for remote skills per the well-known skills discovery spec).
//
// Local skills are loaded from (in order, later overrides earlier):
//
// Global locations (from home directory):
//   - ~/.codex/skills/ (recursive)
//   - ~/.claude/skills/ (flat)
//   - ~/.agents/skills/ (recursive)
//
// Project locations (from git root up to cwd, closest wins):
//   - .claude/skills/ (flat, only at cwd)
//   - .agents/skills/ (flat, scanned from git root to cwd)
func Load(sources []string) []Skill {
	skillMap := make(map[string]Skill)

	for _, source := range sources {
		switch {
		case source == "local":
			for _, skill := range loadLocalSkills() {
				skillMap[skill.Name] = skill
			}
		case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
			for _, skill := range loadRemoteSkills(source) {
				skillMap[source+"/"+skill.Name] = skill
			}
		}
	}

	result := make([]Skill, 0, len(skillMap))
	for _, skill := range skillMap {
		result = append(result, skill)
	}
	return result
}

// loadLocalSkills loads skills from standard filesystem locations.
func loadLocalSkills() []Skill {
	skillMap := make(map[string]Skill)

	homeDir := paths.GetHomeDir()
	if homeDir != "" {
		// Load from codex directory (recursive)
		for _, skill := range loadSkillsFromDir(filepath.Join(homeDir, ".codex", "skills"), true) {
			skillMap[skill.Name] = skill
		}
		// Load from claude user directory (flat)
		for _, skill := range loadSkillsFromDir(filepath.Join(homeDir, ".claude", "skills"), false) {
			skillMap[skill.Name] = skill
		}
		// Load from agents user directory (recursive)
		for _, skill := range loadSkillsFromDir(filepath.Join(homeDir, ".agents", "skills"), true) {
			skillMap[skill.Name] = skill
		}
	}

	// Load from project directories
	if cwd, err := os.Getwd(); err == nil {
		// Load .claude/skills from cwd only (backward compatibility)
		for _, skill := range loadSkillsFromDir(filepath.Join(cwd, ".claude", "skills"), false) {
			skillMap[skill.Name] = skill
		}

		// Load .agents/skills from git root up to cwd (closest wins)
		// We iterate from root to cwd so that later (closer) directories override earlier ones
		for _, dir := range projectSearchDirs(cwd) {
			for _, skill := range loadSkillsFromDir(filepath.Join(dir, ".agents", "skills"), false) {
				skillMap[skill.Name] = skill
			}
		}
	}

	result := make([]Skill, 0, len(skillMap))
	for _, skill := range skillMap {
		result = append(result, skill)
	}
	return result
}

// projectSearchDirs returns directories from git root to cwd (inclusive).
// If not in a git repo, returns only cwd.
// The returned slice is ordered from root to cwd so that closer directories
// can override skills from parent directories.
func projectSearchDirs(cwd string) []string {
	absPath, err := filepath.Abs(cwd)
	if err != nil {
		return []string{cwd}
	}

	// Find git root by walking up
	gitRoot := findGitRoot(absPath)
	if gitRoot == "" {
		// Not in a git repo, just return cwd
		return []string{absPath}
	}

	// Build list of directories from git root to cwd
	var dirs []string
	current := absPath
	for {
		dirs = append(dirs, current)
		if current == gitRoot {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root without finding git root (shouldn't happen)
			break
		}
		current = parent
	}

	// Reverse so we go from root to cwd (earlier entries get overridden by later)
	slices.Reverse(dirs)

	return dirs
}

// findGitRoot finds the git repository root by looking for .git directory or file.
// Returns empty string if not in a git repository.
func findGitRoot(dir string) string {
	current := dir
	for {
		gitPath := filepath.Join(current, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			// .git can be a directory (normal repo) or a file (worktree/submodule)
			if info.IsDir() || info.Mode().IsRegular() {
				return current
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			return ""
		}
		current = parent
	}
}

// loadSkillsFromDir loads skills from a directory.
// If recursive is true, it walks all subdirectories looking for SKILL.md files.
// If recursive is false, it only looks for SKILL.md in immediate subdirectories.
func loadSkillsFromDir(dir string, recursive bool) []Skill {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	if recursive {
		return loadSkillsRecursive(dir)
	}
	return loadSkillsFlat(dir)
}

// loadSkillsFlat loads skills from immediate subdirectories only (Claude format).
func loadSkillsFlat(dir string) []Skill {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var skills []Skill
	for _, entry := range entries {
		if !entry.IsDir() || (isHidden(entry) || isSymlink(entry)) {
			continue
		}

		skillDir := filepath.Join(dir, entry.Name())
		skillFilePath := filepath.Join(skillDir, skillFile)

		skill, ok := loadSkillFile(skillFilePath, entry.Name())
		if ok {
			skills = append(skills, skill)
		}
	}
	return skills
}

// loadSkillsRecursive loads skills from all subdirectories (Codex format).
// It tracks visited real directory paths to avoid infinite loops caused by
// symlinks that form cycles.
func loadSkillsRecursive(dir string) []Skill {
	visited := make(map[string]bool)

	// Resolve the root so cycles back to it are detected.
	if realDir, err := filepath.EvalSymlinks(dir); err == nil {
		visited[realDir] = true
	}

	return walkSkillsRecursive(dir, visited)
}

// walkSkillsRecursive walks dir for SKILL.md files, using visited to skip
// directories whose real path has already been traversed.
func walkSkillsRecursive(dir string, visited map[string]bool) []Skill {
	var skills []Skill

	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if path != dir && isHidden(d) {
				return fs.SkipDir
			}

			// Resolve and de-duplicate real directory paths to catch
			// cycles introduced through symlinks higher up.
			if path != dir {
				if realPath, err := filepath.EvalSymlinks(path); err == nil {
					if visited[realPath] {
						return fs.SkipDir
					}
					visited[realPath] = true
				}
			}
			return nil
		}

		if d.Name() != skillFile {
			return nil
		}

		skillDir := filepath.Dir(path)
		dirName := filepath.Base(skillDir)

		if skill, ok := loadSkillFile(path, dirName); ok {
			skills = append(skills, skill)
		}
		return nil
	})

	return skills
}

// loadSkillFile reads and parses a SKILL.md file.
func loadSkillFile(path, dirName string) (Skill, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, false
	}

	skill, ok := parseFrontmatter(string(content))
	if !ok {
		return Skill{}, false
	}

	skill.Name = cmp.Or(skill.Name, dirName)

	if !isValidSkill(skill) {
		return Skill{}, false
	}
	skill.FilePath = path
	skill.BaseDir = filepath.Dir(path)
	skill.Local = true

	return skill, true
}

// parseFrontmatter extracts and parses the YAML-like frontmatter from a
// markdown file. Instead of using a full YAML parser (which rejects unquoted
// colons in values), we do simple line-by-line key: value splitting on the
// first ": ". This is more robust for the simple frontmatter format used by
// skill files.
func parseFrontmatter(content string) (Skill, bool) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	if !strings.HasPrefix(content, "---") {
		return Skill{}, false
	}

	endIndex := strings.Index(content[3:], "\n---")
	if endIndex == -1 {
		return Skill{}, false
	}

	block := content[4 : endIndex+3]
	lines := strings.Split(block, "\n")

	var skill Skill
	var currentKey string // tracks multi-line keys like "metadata" or "allowed-tools"

	for _, line := range lines {
		// Indented lines belong to the current multi-line key.
		if line != "" && (line[0] == ' ' || line[0] == '\t') {
			trimmed := strings.TrimSpace(line)
			switch currentKey {
			case "metadata":
				if k, v, ok := splitKeyValue(trimmed); ok {
					if skill.Metadata == nil {
						skill.Metadata = make(map[string]string)
					}
					skill.Metadata[k] = unquote(v)
				}
			case "allowed-tools":
				if strings.HasPrefix(trimmed, "- ") {
					skill.AllowedTools = append(skill.AllowedTools, unquote(strings.TrimSpace(trimmed[2:])))
				}
			}
			continue
		}

		currentKey = ""
		key, value, ok := splitKeyValue(line)
		if !ok {
			continue
		}

		switch key {
		case "name":
			skill.Name = unquote(value)
		case "description":
			skill.Description = unquote(value)
		case "license":
			skill.License = unquote(value)
		case "compatibility":
			skill.Compatibility = unquote(value)
		case "context":
			skill.Context = unquote(value)
		case "model":
			skill.Model = unquote(value)
		case "metadata":
			currentKey = "metadata"
		case "allowed-tools":
			if value != "" {
				// Inline comma-separated list.
				for item := range strings.SplitSeq(value, ",") {
					if t := unquote(strings.TrimSpace(item)); t != "" {
						skill.AllowedTools = append(skill.AllowedTools, t)
					}
				}
			} else {
				currentKey = "allowed-tools"
			}
		}
	}

	return skill, true
}

// splitKeyValue splits a line on the first ": " into key and value.
func splitKeyValue(line string) (string, string, bool) {
	if key, value, ok := strings.Cut(line, ": "); ok {
		return key, value, true
	}
	// Handle "key:" with no value (e.g. "metadata:").
	if strings.HasSuffix(line, ":") {
		return line[:len(line)-1], "", true
	}
	return "", "", false
}

// unquote strips matching surrounding quotes from a string value.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func isValidSkill(skill Skill) bool {
	return skill.Description != "" && skill.Name != ""
}

func isHidden(entry fs.DirEntry) bool {
	return strings.HasPrefix(entry.Name(), ".")
}

func isSymlink(entry fs.DirEntry) bool {
	return entry.Type()&os.ModeSymlink != 0
}
