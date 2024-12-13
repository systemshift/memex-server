package utils

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateNodeType checks if a node type is valid against a list of valid types
func ValidateNodeType(nodeType string, validTypes []string) bool {
	for _, valid := range validTypes {
		if valid == nodeType {
			return true
		}
		// Support glob patterns (e.g., "ast.*")
		if strings.Contains(valid, "*") {
			if matched, _ := filepath.Match(valid, nodeType); matched {
				return true
			}
		}
	}
	return false
}

// IsNodeTypeValid checks if a node type matches a pattern
func IsNodeTypeValid(nodeType string, pattern string) bool {
	if pattern == "*" {
		return true
	}
	matched, _ := filepath.Match(pattern, nodeType)
	return matched
}

// ValidateLinkType checks if a link type is valid against a list of valid types
func ValidateLinkType(linkType string, validTypes []string) bool {
	for _, valid := range validTypes {
		if valid == linkType {
			return true
		}
		// Support glob patterns (e.g., "ast.*")
		if strings.Contains(valid, "*") {
			if matched, _ := filepath.Match(valid, linkType); matched {
				return true
			}
		}
	}
	return false
}

// IsLinkTypeValid checks if a link type matches a pattern
func IsLinkTypeValid(linkType string, pattern string) bool {
	if pattern == "*" {
		return true
	}
	matched, _ := filepath.Match(pattern, linkType)
	return matched
}

// ValidateMetadata checks if required metadata fields are present
func ValidateMetadata(meta map[string]interface{}, required []string) error {
	for _, field := range required {
		if _, ok := meta[field]; !ok {
			return fmt.Errorf("missing required metadata field: %s", field)
		}
	}
	return nil
}

// MergeMetadata merges override metadata into base metadata
func MergeMetadata(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy base metadata
	for k, v := range base {
		result[k] = v
	}

	// Apply overrides
	for k, v := range override {
		result[k] = v
	}

	return result
}
