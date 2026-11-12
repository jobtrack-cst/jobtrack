package domain

import "strings"

// Kind selects one of the three seeded reference lists.
type Kind uint8

const (
	KindUnspecified Kind = iota
	KindCategory
	KindTag
	KindLocation
)

func (k Kind) String() string {
	switch k {
	case KindCategory:
		return "category"
	case KindTag:
		return "tag"
	case KindLocation:
		return "location"
	default:
		return ""
	}
}

// Item is one seeded category, tag or location. Names are 1 to 100
// characters, already trimmed and unique ignoring case; the database enforces
// this, because rows only come from migrations.
type Item struct {
	ID   string
	Name string
}

// MaxNameLength matches the job-posting location limit (JT-38), so every
// seeded location name fits in a job.
const MaxNameLength = 100

// MaxResolveIDs bounds each ResolveNames list. job-alert sends at most 20.
const MaxResolveIDs = 100

// ResolveIDs prepares one ResolveNames list. More than MaxResolveIDs entries
// is ErrTooManyIDs. Entries that are not UUIDs are dropped like unknown IDs,
// because the contract leaves unknown IDs out instead of failing; job-alert
// then rejects them itself. IDs are lowercased and duplicates removed,
// keeping the first occurrence.
func ResolveIDs(raw []string) ([]string, error) {
	if len(raw) > MaxResolveIDs {
		return nil, ErrTooManyIDs
	}
	seen := make(map[string]bool, len(raw))
	ids := []string{}
	for _, id := range raw {
		if !IsUUID(id) {
			continue
		}
		id = strings.ToLower(id)
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}
