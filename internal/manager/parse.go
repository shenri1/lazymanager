package manager

import "strings"

// ParseList turns package-manager output into packages. It accepts "name
// version", "name==version", "name@version" and npm-style tree lines.
func ParseList(out string) []Package {
	var pkgs []Package
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "├└─│`+ ")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Listing") ||
			strings.HasPrefix(line, "Installed") ||
			strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) == 1 && strings.HasSuffix(line, ":") {
			continue // section header such as "dependencies:"
		}
		if line == "(empty)" || (len(fields) > 1 && strings.HasPrefix(fields[1], "/")) {
			continue // npm/pnpm: empty tree marker or "project@1.0 /path" root line
		}

		var name, version string
		if i := strings.Index(line, "=="); i > 0 {
			name = strings.TrimSpace(line[:i])
			version = strings.TrimSpace(line[i+2:])
		} else {
			name = fields[0]
			if len(fields) > 1 {
				version = fields[1]
			}
		}
		if version == "" {
			if i := strings.LastIndex(name, "@"); i > 0 {
				version = name[i+1:]
				name = name[:i]
			}
		}
		if name == "" {
			continue
		}
		pkgs = append(pkgs, Package{Name: name, Version: version})
	}
	return pkgs
}

// ParseOutdated turns update-listing output into packages. The new version is
// the last field when an arrow ("->" or "<") is present, otherwise the field
// at col (where 0 means field 1).
func ParseOutdated(out string, col int) []Package {
	if col == 0 {
		col = 1
	}
	var pkgs []Package
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "├└─│`+ ")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "-") ||
			strings.HasPrefix(line, "=") ||
			strings.HasPrefix(line, "|") ||
			strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, "Listing") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "Package" || fields[0] == "Name" {
			continue
		}
		name := fields[0]
		version := ""
		if hasArrow(fields) {
			version = fields[len(fields)-1]
		} else if col < len(fields) {
			version = fields[col]
		}
		if name == "" {
			continue
		}
		pkgs = append(pkgs, Package{Name: name, Version: version})
	}
	return pkgs
}

func hasArrow(fields []string) bool {
	for _, f := range fields {
		if f == "->" || f == "<" {
			return true
		}
	}
	return false
}
