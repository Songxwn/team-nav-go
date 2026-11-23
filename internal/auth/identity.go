package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func sha256Sum(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// claimString reads a string claim from decoded ID token claims.
func claimString(claims map[string]any, name string) string {
	if name == "" {
		return ""
	}
	if v, ok := claims[name]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// claimGroups accepts both the single string and array forms used by identity
// providers for group claims.
func claimGroups(claims map[string]any, name string) []string {
	if name == "" {
		return nil
	}
	raw, ok := claims[name]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func containsGroup(groups []string, wanted string) bool {
	wanted = strings.TrimSpace(wanted)
	if wanted == "" {
		return false
	}
	for _, group := range groups {
		if strings.EqualFold(strings.TrimSpace(group), wanted) {
			return true
		}
	}
	return false
}
