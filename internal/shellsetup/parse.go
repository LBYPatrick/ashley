package shellsetup

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var assignment = regexp.MustCompile(`(?s)^(?:export[ \t]+)?(PATH|PNPM_HOME|path)(\+?=)(.*)$`)

func existingSettings(cfg Config, shell, content string) (map[string]bool, bool) {
	paths := make(map[string]bool)
	variables := map[string]string{"HOME": cfg.Home, "PNPM_HOME": cfg.PNPMHome}
	pnpmSet := false
	addPaths := func(words []string) {
		for _, word := range words {
			for _, path := range strings.Split(word, ":") {
				if filepath.IsAbs(path) && !strings.ContainsRune(path, '\x00') {
					paths[filepath.Clean(path)] = true
				}
			}
		}
	}
	for _, statement := range shellStatements(content, shell == "fish") {
		if shell == "fish" {
			words := shellWords(statement, variables, true)
			if len(words) == 0 {
				continue
			}
			switch words[0] {
			case "fish_add_path":
				if containsAny(words[1:], "--dry-run", "-n") {
					continue
				}
				for _, word := range words[1:] {
					if !strings.HasPrefix(word, "-") {
						addPaths([]string{word})
					}
				}
			case "set":
				args := words[1:]
				query := false
				for len(args) > 0 && strings.HasPrefix(args[0], "-") {
					flag := args[0]
					if flag == "--query" || flag == "--erase" || (strings.HasPrefix(flag, "-") && !strings.HasPrefix(flag, "--") && strings.ContainsAny(flag, "qe")) {
						query = true
					}
					args = args[1:]
				}
				if query {
					continue
				}
				if len(args) > 1 && args[0] == "PNPM_HOME" {
					pnpmSet = true
					variables["PNPM_HOME"] = args[1]
				} else if len(args) > 1 && args[0] == "PATH" {
					addPaths(args[1:])
				}
			}
			continue
		}
		match := assignment.FindStringSubmatch(statement)
		if match == nil {
			continue
		}
		name, value := match[1], match[3]
		if name == "path" {
			if shell != "zsh" || !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
				continue
			}
			addPaths(shellWords(value[1:len(value)-1], variables, false))
			continue
		}
		words := shellWords(value, variables, false)
		if !strings.HasPrefix(statement, "export") && len(words) > 1 {
			continue
		}
		if name == "PNPM_HOME" {
			pnpmSet = true
			variables[name] = ""
		}
		if len(words) == 0 {
			continue
		}
		if name == "PNPM_HOME" {
			variables[name] = words[0]
		} else {
			addPaths(words[:1])
		}
	}
	return paths, pnpmSet
}

func containsAny(words []string, values ...string) bool {
	for _, word := range words {
		for _, value := range values {
			if word == value {
				return true
			}
		}
	}
	return false
}

// shellStatements separates simple commands without interpreting shell code.
func shellStatements(content string, fish bool) []string {
	var statements []string
	var current strings.Builder
	var quote byte
	escaped, comment := false, false
	depth := 0
	flush := func() {
		if statement := strings.TrimSpace(current.String()); statement != "" {
			statements = append(statements, statement)
		}
		current.Reset()
	}
	for i := 0; i < len(content); i++ {
		ch := content[i]
		if comment {
			if ch != '\n' {
				continue
			}
			comment = false
		}
		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' && (quote != '\'' || (fish && i+1 < len(content) && (content[i+1] == '\'' || content[i+1] == '\\'))) {
			current.WriteByte(ch)
			escaped = true
			continue
		}
		if quote != 0 {
			current.WriteByte(ch)
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
		} else if ch == '#' && (current.Len() == 0 || strings.ContainsRune(" \t\n;(", rune(content[i-1]))) {
			comment = true
			continue
		} else if ch == '(' {
			depth++
		} else if ch == ')' && depth > 0 {
			depth--
		} else if (ch == ';' || ch == '\n') && depth == 0 {
			flush()
			continue
		}
		current.WriteByte(ch)
	}
	flush()
	return statements
}

func shellWords(value string, variables map[string]string, fish bool) []string {
	var words []string
	var word strings.Builder
	var quote byte
	started := false
	flush := func() {
		if started {
			words = append(words, word.String())
		}
		word.Reset()
		started = false
	}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch == '\\' && i+1 < len(value) && (quote != '\'' || fish) {
			next := value[i+1]
			if quote == 0 || (quote == '"' && strings.ContainsRune("$`\"\\\n", rune(next))) || (fish && quote == '\'' && (next == '\'' || next == '\\')) {
				i++
				if next != '\n' {
					word.WriteByte(next)
					started = true
				}
				continue
			}
		}
		if quote != 0 && ch == quote {
			quote = 0
			continue
		}
		if quote == 0 && (ch == '\'' || ch == '"') {
			quote = ch
			started = true
			continue
		}
		if quote == 0 && unicode.IsSpace(rune(ch)) {
			flush()
			continue
		}
		if ch == '$' && quote != '\'' {
			expanded, consumed := variable(value[i:], variables)
			word.WriteString(expanded)
			i += consumed - 1
		} else if ch == '~' && quote == 0 && (word.Len() == 0 || strings.HasSuffix(word.String(), ":")) && (i+1 == len(value) || value[i+1] == '/') {
			word.WriteString(variables["HOME"])
		} else {
			word.WriteByte(ch)
		}
		started = true
	}
	flush()
	return words
}

func variable(value string, variables map[string]string) (string, int) {
	name, consumed := "", 1
	if len(value) > 1 && value[1] == '{' {
		if end := strings.IndexByte(value, '}'); end >= 0 {
			name, consumed = value[2:end], end+1
		}
	} else {
		for consumed < len(value) {
			ch := value[consumed]
			if ch != '_' && (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') {
				break
			}
			consumed++
		}
		name = value[1:consumed]
	}
	if name == "PATH" || name == "path" || strings.HasPrefix(name, "PATH:") || name == "PATH-" {
		return "", consumed
	}
	if expanded, ok := variables[name]; ok && expanded != "" {
		return expanded, consumed
	}
	return "\x00", consumed
}
