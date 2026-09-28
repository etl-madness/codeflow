package discovery

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// Supported language identifiers.
const (
	LangGo        = "go"
	LangCSharp    = "csharp"
	LangPython    = "python"
	LangSQL       = "sql"
	LangFlowXML   = "flowxml"
	LangSSIS      = "ssis"
	LangLiquibase = "liquibase"
)

// CommonTestPatterns provides default ignore patterns for test and spec files across supported languages.
var CommonTestPatterns = []string{
	"*_test.go",
	"test_*.py",
	"*_test.py",
	"*Test.cs",
	"*Tests.cs",
	"*.Test.cs",
	"*.Tests.cs",
	"*.test.js",
	"*.spec.js",
	"*.test.ts",
	"*.spec.ts",
	"*.test.jsx",
	"*.spec.jsx",
	"*.test.tsx",
	"*.spec.tsx",
}

// DiscoveredFile contains metadata about a recognized source file.
type DiscoveredFile struct {
	Path      string
	Extension string
	Language  string
}

// Scanner handles filesystem traversal and file filtering.
type Scanner struct {
	rootDir      string
	recursive    bool
	absExcludes  []string
	relExcludes  []string
	nameExcludes []string
	filePatterns []string
	ignorer      *gitignore.GitIgnore
}

var defaultIgnorePatterns = []string{
	".git",
	".svn",
	".hg",
	".idea",
	".vscode",
	"node_modules",
	"vendor",
	"bin",
	"obj",
	"__pycache__",
	"*.pyc",
	".gemini",
}

// NewScanner creates a new Scanner instance for the given root directory.
// Optional excludes can be provided as folder names, relative paths (to cwd or to source root),
// absolute paths, or file glob patterns (e.g. *_test.go, test_*.py, *.spec.ts).
func NewScanner(rootDir string, recursive bool, excludes ...string) (*Scanner, error) {
	return NewScannerWithOptions(rootDir, recursive, false, excludes...)
}

// NewScannerWithOptions creates a new Scanner instance with configurable .gitignore handling.
func NewScannerWithOptions(rootDir string, recursive bool, noGitignore bool, excludes ...string) (*Scanner, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	absRoot = filepath.Clean(absRoot)

	patterns := append([]string{}, defaultIgnorePatterns...)

	var absExcludes []string
	var relExcludes []string
	var nameExcludes []string
	var filePatterns []string

	for _, ed := range excludes {
		for _, part := range strings.Split(ed, ",") {
			raw := strings.TrimSpace(part)
			raw = strings.Trim(raw, "\"'")
			if raw == "" {
				continue
			}

			// Check if raw contains wildcard characters (*, ?, [)
			if strings.ContainsAny(raw, "*?[") {
				normPattern := filepath.ToSlash(raw)
				filePatterns = append(filePatterns, normPattern)
				basePat := path.Base(normPattern)
				if basePat != "." && basePat != "/" && basePat != "" {
					filePatterns = append(filePatterns, basePat)
				}
				patterns = append(patterns, normPattern, "**/"+normPattern)
				continue
			}

			cleanPath := filepath.Clean(raw)

			// 1. Resolve relative to current working directory or as absolute path
			absTarget, err := filepath.Abs(cleanPath)
			if err == nil {
				cleanedAbs := filepath.Clean(absTarget)
				absExcludes = append(absExcludes, cleanedAbs)

				// Check if absTarget is inside or equal to absRoot
				relFromRoot, err := filepath.Rel(absRoot, cleanedAbs)
				if err == nil && !strings.HasPrefix(relFromRoot, "..") && !filepath.IsAbs(relFromRoot) && relFromRoot != "." {
					slashRel := filepath.ToSlash(relFromRoot)
					relExcludes = append(relExcludes, slashRel)
					patterns = append(patterns, slashRel, slashRel+"/", "**/"+slashRel, "**/"+slashRel+"/**")
				}
			}

			// 2. Resolve relative to absRoot (e.g. user provided "examples" or "sub/dir")
			relTarget := filepath.Clean(filepath.Join(absRoot, cleanPath))
			absExcludes = append(absExcludes, relTarget)
			if relFromRoot, err := filepath.Rel(absRoot, relTarget); err == nil && !strings.HasPrefix(relFromRoot, "..") && !filepath.IsAbs(relFromRoot) && relFromRoot != "." {
				slashRel := filepath.ToSlash(relFromRoot)
				relExcludes = append(relExcludes, slashRel)
				patterns = append(patterns, slashRel, slashRel+"/", "**/"+slashRel, "**/"+slashRel+"/**")
			}

			// 3. Name or base pattern match
			baseName := filepath.Base(cleanPath)
			if baseName != "." && baseName != "/" && baseName != "\\" && baseName != "" {
				nameExcludes = append(nameExcludes, baseName)
				patterns = append(patterns, baseName, baseName+"/", "**/"+baseName, "**/"+baseName+"/**")
			}
		}
	}

	// Look for .gitignore in rootDir unless noGitignore is specified
	if !noGitignore {
		gitignorePath := filepath.Join(absRoot, ".gitignore")
		if _, err := os.Stat(gitignorePath); err == nil {
			gi, err := gitignore.CompileIgnoreFileAndLines(gitignorePath, patterns...)
			if err == nil {
				return &Scanner{
					rootDir:      absRoot,
					recursive:    recursive,
					absExcludes:  absExcludes,
					relExcludes:  relExcludes,
					nameExcludes: nameExcludes,
					filePatterns: filePatterns,
					ignorer:      gi,
				}, nil
			}
		}
	}

	gi := gitignore.CompileIgnoreLines(patterns...)
	return &Scanner{
		rootDir:      absRoot,
		recursive:    recursive,
		absExcludes:  absExcludes,
		relExcludes:  relExcludes,
		nameExcludes: nameExcludes,
		filePatterns: filePatterns,
		ignorer:      gi,
	}, nil
}

// MapExtensionToLanguage maps a file extension to its recognized canonical language.
func MapExtensionToLanguage(ext string) (string, bool) {
	switch strings.ToLower(ext) {
	case ".go":
		return LangGo, true
	case ".cs", ".razor":
		return LangCSharp, true
	case ".py":
		return LangPython, true
	case ".sql":
		return LangSQL, true
	case ".xml":
		return LangFlowXML, true
	case ".dtsx":
		return LangSSIS, true
	default:
		return "", false
	}
}

// Scan performs filesystem discovery and returns matching source files.
func (s *Scanner) Scan() ([]DiscoveredFile, error) {
	var discovered []DiscoveredFile

	rootInfo, err := os.Stat(s.rootDir)
	if err != nil {
		return nil, err
	}

	// Single-file scan target
	if !rootInfo.IsDir() {
		ext := filepath.Ext(s.rootDir)
		if lang, ok := MapExtensionToLanguage(ext); ok {
			detectedLang := sniffLanguage(s.rootDir, lang)
			discovered = append(discovered, DiscoveredFile{
				Path:      s.rootDir,
				Extension: strings.ToLower(ext),
				Language:  detectedLang,
			})
		}
		return discovered, nil
	}

	err = filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip unreadable paths gracefully
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil
		}

		// Calculate relative path for gitignore evaluation
		relPath, err := filepath.Rel(s.rootDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		// Normalize path separators to forward slash for matching
		normalizedRel := filepath.ToSlash(relPath)
		baseName := filepath.Base(path)

		if info.IsDir() {
			if s.isExcluded(absPath, normalizedRel, baseName) {
				return filepath.SkipDir
			}
			if s.ignorer != nil && s.ignorer.MatchesPath(normalizedRel) {
				return filepath.SkipDir
			}
			if !s.recursive && filepath.Dir(relPath) != "." {
				return filepath.SkipDir
			}
			return nil
		}

		if s.isExcluded(absPath, normalizedRel, baseName) {
			return nil
		}

		if s.ignorer != nil && s.ignorer.MatchesPath(normalizedRel) {
			return nil
		}

		ext := filepath.Ext(path)
		if lang, ok := MapExtensionToLanguage(ext); ok {
			detectedLang := sniffLanguage(path, lang)
			discovered = append(discovered, DiscoveredFile{
				Path:      path,
				Extension: strings.ToLower(ext),
				Language:  detectedLang,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return discovered, nil
}

func (s *Scanner) isExcluded(absPath, normalizedRel, baseName string) bool {
	lowerAbs := strings.ToLower(absPath)
	lowerRel := strings.ToLower(normalizedRel)
	lowerBase := strings.ToLower(baseName)

	// 1. Check file glob patterns (e.g. *_test.go, test_*.py, *.spec.ts, tests/*)
	for _, pat := range s.filePatterns {
		if matchPattern(pat, lowerBase) || matchPattern(pat, lowerRel) {
			return true
		}
	}

	// 2. Check absolute path matches
	for _, absExc := range s.absExcludes {
		lowerExc := strings.ToLower(absExc)
		if lowerAbs == lowerExc {
			return true
		}
		if strings.HasPrefix(lowerAbs, lowerExc+string(filepath.Separator)) ||
			strings.HasPrefix(lowerAbs, lowerExc+"/") {
			return true
		}
	}

	// 3. Check relative path matches
	for _, relExc := range s.relExcludes {
		lowerExc := strings.ToLower(relExc)
		if lowerRel == lowerExc {
			return true
		}
		if strings.HasPrefix(lowerRel, lowerExc+"/") {
			return true
		}
	}

	// 4. Check name / segment matches
	parts := strings.Split(lowerRel, "/")
	for _, nameExc := range s.nameExcludes {
		lowerExc := strings.ToLower(nameExc)
		if lowerBase == lowerExc {
			return true
		}
		for _, part := range parts {
			if part == lowerExc {
				return true
			}
		}
		if matchPattern(lowerExc, lowerBase) {
			return true
		}
	}

	return false
}

// matchPattern evaluates glob patterns including wildcards, base name matching, and double-star (**).
func matchPattern(pattern, target string) bool {
	pattern = strings.ToLower(filepath.ToSlash(pattern))
	target = strings.ToLower(filepath.ToSlash(target))

	if pattern == target {
		return true
	}

	// 1. Standard path.Match
	if matched, _ := path.Match(pattern, target); matched {
		return true
	}

	// 2. Base name match if pattern has no directory slashes
	if !strings.Contains(pattern, "/") {
		base := path.Base(target)
		if matched, _ := path.Match(pattern, base); matched {
			return true
		}
	}

	// 3. Double-star glob handling (e.g. **/tests/**, **/*_test.go, tests/**)
	if strings.Contains(pattern, "**") {
		regexPattern := regexp.QuoteMeta(pattern)
		regexPattern = strings.ReplaceAll(regexPattern, `\*\*`, `.*`)
		regexPattern = strings.ReplaceAll(regexPattern, `\*`, `[^/]*`)
		regexPattern = strings.ReplaceAll(regexPattern, `\?`, `[^/]`)
		if re, err := regexp.Compile("^" + regexPattern + "$"); err == nil {
			if re.MatchString(target) {
				return true
			}
		}
	}

	// 4. Wildcard prefix/suffix convenience
	// e.g. pattern "*_test.go" on "auth_test.go" or "pkg/auth_test.go"
	if strings.HasPrefix(pattern, "*") && !strings.Contains(pattern[1:], "*") && !strings.Contains(pattern, "/") {
		suffix := pattern[1:]
		if strings.HasSuffix(target, suffix) {
			return true
		}
	}
	if strings.HasSuffix(pattern, "*") && !strings.Contains(pattern[:len(pattern)-1], "*") && !strings.Contains(pattern, "/") {
		prefix := pattern[:len(pattern)-1]
		base := path.Base(target)
		if strings.HasPrefix(base, prefix) {
			return true
		}
	}

	return false
}

// sniffLanguage inspects the initial bytes of XML and SQL files to distinguish Liquibase changelogs and SSIS packages.
func sniffLanguage(filePath string, defaultLang string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".xml" || ext == ".sql" {
		f, err := os.Open(filePath)
		if err != nil {
			return defaultLang
		}
		defer f.Close()

		buf := make([]byte, 1024)
		n, _ := f.Read(buf)
		header := buf[:n]

		if ext == ".xml" {
			if bytes.Contains(header, []byte("<databaseChangeLog")) {
				return LangLiquibase
			}
			if bytes.Contains(header, []byte("<DTS:Executable")) || bytes.Contains(header, []byte("www.microsoft.com/SqlServer/Dts")) {
				return LangSSIS
			}
		} else if ext == ".sql" {
			if bytes.Contains(header, []byte("--liquibase formatted sql")) || bytes.Contains(header, []byte("--changeset ")) {
				return LangLiquibase
			}
		}
	}
	return defaultLang
}
