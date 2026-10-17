package updater

import (
	"fmt"
	"regexp"
	"strconv"
)

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(alpha|beta|rc)\.(0|[1-9][0-9]*))?$`)

type version struct {
	text     string
	parts    [3]uint64
	stage    string
	sequence uint64
}

func parseVersion(text string, stable bool) (version, error) {
	match := versionPattern.FindStringSubmatch(text)
	if match == nil || (stable && match[4] != "") {
		return version{}, fmt.Errorf("invalid %s version %q", map[bool]string{true: "stable", false: "semantic"}[stable], text)
	}
	v := version{text: text, stage: match[4]}
	for i := range v.parts {
		n, err := strconv.ParseUint(match[i+1], 10, 64)
		if err != nil {
			return version{}, fmt.Errorf("version component is too large: %q", text)
		}
		v.parts[i] = n
	}
	if v.stage != "" {
		n, err := strconv.ParseUint(match[5], 10, 64)
		if err != nil {
			return version{}, fmt.Errorf("version component is too large: %q", text)
		}
		v.sequence = n
	}
	return v, nil
}

func (v version) compare(other version) int {
	for i, n := range v.parts {
		if n < other.parts[i] {
			return -1
		}
		if n > other.parts[i] {
			return 1
		}
	}
	rank := map[string]int{"alpha": 0, "beta": 1, "rc": 2, "": 3}
	if rank[v.stage] < rank[other.stage] {
		return -1
	}
	if rank[v.stage] > rank[other.stage] {
		return 1
	}
	if v.sequence < other.sequence {
		return -1
	}
	if v.sequence > other.sequence {
		return 1
	}
	return 0
}

type bounds struct{ minimum, maximum *version }

func parseBounds(minimum, maximum string) (bounds, error) {
	var b bounds
	for _, item := range []struct {
		text   string
		target **version
	}{{minimum, &b.minimum}, {maximum, &b.maximum}} {
		if item.text == "" {
			continue
		}
		v, err := parseVersion(item.text, true)
		if err != nil {
			return b, err
		}
		*item.target = &v
	}
	if b.minimum != nil && b.maximum != nil && b.minimum.compare(*b.maximum) >= 0 {
		return b, fmt.Errorf("min-version must be less than max-version (exclusive)")
	}
	return b, nil
}

func (b bounds) contains(v version) bool {
	return (b.minimum == nil || v.compare(*b.minimum) >= 0) && (b.maximum == nil || v.compare(*b.maximum) < 0)
}
