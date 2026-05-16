package palette

import (
	"strings"
	"unicode"
)

// Collapse groups names that share a common prefix + suffix-shape into
// a single "<prefix>-*" row. The rule is:
//
//	"<prefix>-<short token>"  →  group by prefix + token-shape
//
// where a "short token" is 1–10 chars and the shape is one of:
//
//	numeric   — all digits        (kafka-0, kafka-1)
//	alpha     — all letters/digits (nginx-7c8d4 → split further, see below)
//	mixed     — anything else      (rarely collapses)
//
// Two members must agree on BOTH the prefix AND the suffix shape
// before they merge — that's what stops "kafka-0" and
// "kafka-broker-1" from accidentally collapsing into "kafka-*".
//
// For long pod names like "nginx-deployment-7c8d4-abc12" Collapse
// folds them by stripping the LAST short alphanumeric segment
// ("abc12"), producing "nginx-deployment-7c8d4-*". The deployment-
// replicaset-pod naming convention sits comfortably inside this shape.
//
// `min` is the threshold for collapsing — groups with fewer members
// than min are emitted as individual rows. K8s defaults to 2 (collapse
// aggressively); Solace defaults higher.
func Collapse(names []string, min int) []Group {
	if min < 2 {
		min = 2
	}

	type key struct {
		prefix string
		shape  shapeKind
	}
	buckets := map[key][]Resource{}
	singles := []Group{}

	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		prefix, suffix := splitLastToken(name)
		if prefix == "" || suffix == "" {
			singles = append(singles, Group{Display: name, Members: []Resource{{Name: name}}})
			continue
		}
		shape := classify(suffix)
		k := key{prefix: prefix, shape: shape}
		buckets[k] = append(buckets[k], Resource{Name: name})
	}

	out := singles
	for k, members := range buckets {
		if len(members) < min {
			for _, m := range members {
				out = append(out, Group{Display: m.Name, Members: []Resource{m}})
			}
			continue
		}
		// Stable order of members so the popup is deterministic.
		sortMembersByName(members)
		out = append(out, Group{
			Display: k.prefix + "-*",
			Members: members,
		})
	}

	SortGroupsByDisplay(out)
	return out
}

// shapeKind is the suffix-character classification — two suffixes
// with the same shape are "the same kind of thing" for collapse
// purposes.
type shapeKind int

const (
	shapeEmpty shapeKind = iota
	shapeNumeric
	shapeAlpha   // letters + optional digits (5 chars random suffix)
	shapeMixed   // anything else; rarely collapses with anything
)

func classify(s string) shapeKind {
	if s == "" {
		return shapeEmpty
	}
	hasLetter, hasDigit, hasOther := false, false, false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			hasOther = true
		}
	}
	switch {
	case hasOther:
		return shapeMixed
	case hasLetter:
		return shapeAlpha
	case hasDigit:
		return shapeNumeric
	}
	return shapeEmpty
}

// splitLastToken returns (everything-before-the-last-dash, last-token)
// when the last token looks like a generated identifier — i.e.
// 1–10 alphanumeric characters AND contains at least one digit.
// Anything else returns ("", "") so the name emits as a standalone row.
//
// The "at least one digit" rule is the practical sieve between
// generated suffixes (k8s pod hashes "abc12", ordinals "0",
// replicaset hashes "7c8d4") and meaningful trailing words
// ("frontend", "backend", "gateway") that callers really want to
// keep distinct. The 10-char cap on top of that catches longer
// non-random tails ("notifications", "orchestrator") even when
// they happen to contain a digit.
func splitLastToken(name string) (string, string) {
	dash := strings.LastIndexByte(name, '-')
	if dash <= 0 || dash == len(name)-1 {
		return "", ""
	}
	suffix := name[dash+1:]
	if len(suffix) < 1 || len(suffix) > 10 {
		return "", ""
	}
	hasDigit := false
	for _, r := range suffix {
		switch {
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsLetter(r):
			// fine
		default:
			return "", ""
		}
	}
	if !hasDigit {
		return "", ""
	}
	return name[:dash], suffix
}

func sortMembersByName(rs []Resource) {
	// Tiny len ≤ 50 or so in practice; insertion sort would do, but
	// sort.Slice is fine and not on a hot path.
	for i := 1; i < len(rs); i++ {
		j := i
		for j > 0 && rs[j-1].Name > rs[j].Name {
			rs[j-1], rs[j] = rs[j], rs[j-1]
			j--
		}
	}
}
