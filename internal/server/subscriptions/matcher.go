package subscriptions

import (
	"context"
	"log"
	"strings"
)

// Matcher evaluates events against subscription patterns
type Matcher struct {
	repo Repository
}

// NewMatcher creates a new pattern matcher
func NewMatcher(repo Repository) *Matcher {
	return &Matcher{repo: repo}
}

// Match evaluates if an event matches a subscription pattern
// Returns (matched, cypherResults)
func (m *Matcher) Match(ctx context.Context, event Event, pattern SubscriptionPattern) (bool, []map[string]interface{}) {
	// First check simple patterns (fast, in Go)
	if !m.matchSimple(event, pattern) {
		return false, nil
	}

	// If there's a Cypher pattern, evaluate it
	if pattern.Cypher != "" {
		results, err := m.matchCypher(ctx, event, pattern.Cypher)
		if err != nil {
			log.Printf("Cypher pattern match error: %v", err)
			return false, nil
		}
		// Cypher must return at least one result to match
		if len(results) == 0 {
			return false, nil
		}
		return true, results
	}

	return true, nil
}

// matchSimple evaluates simple pattern criteria (in Go, no database)
func (m *Matcher) matchSimple(event Event, pattern SubscriptionPattern) bool {
	// Check event types
	if len(pattern.EventTypes) > 0 {
		matched := false
		for _, et := range pattern.EventTypes {
			if et == event.Type {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check node types (only for node events)
	if len(pattern.NodeTypes) > 0 && event.NodeType != "" {
		matched := false
		for _, nt := range pattern.NodeTypes {
			if nt == event.NodeType {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check link types (only for link events)
	if len(pattern.LinkTypes) > 0 && event.LinkType != "" {
		matched := false
		for _, lt := range pattern.LinkTypes {
			if lt == event.LinkType {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check metadata matching
	if len(pattern.MetaMatch) > 0 && event.Meta != nil {
		for key, expectedValue := range pattern.MetaMatch {
			actualValue, exists := event.Meta[key]
			if !exists {
				return false
			}
			if !matchValue(expectedValue, actualValue) {
				return false
			}
		}
	}

	return true
}

// matchCypher evaluates a Cypher query pattern
func (m *Matcher) matchCypher(ctx context.Context, event Event, cypher string) ([]map[string]interface{}, error) {
	// Validate Cypher query for safety
	if err := validateCypher(cypher); err != nil {
		return nil, err
	}

	// Build parameters from event context
	params := map[string]interface{}{
		"event_node_id":    event.NodeID,
		"event_node_type":  event.NodeType,
		"event_link_source": event.LinkSource,
		"event_link_target": event.LinkTarget,
		"event_link_type":  event.LinkType,
	}

	// Execute the query
	results, err := m.repo.ExecuteCypherRead(ctx, cypher, params)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// matchValue compares expected and actual values with type flexibility
func matchValue(expected, actual interface{}) bool {
	// Direct equality
	if expected == actual {
		return true
	}

	// String comparison (case-insensitive)
	expectedStr, ok1 := expected.(string)
	actualStr, ok2 := actual.(string)
	if ok1 && ok2 {
		return strings.EqualFold(expectedStr, actualStr)
	}

	// Numeric comparison with type coercion
	expectedNum, ok1 := toFloat64(expected)
	actualNum, ok2 := toFloat64(actual)
	if ok1 && ok2 {
		return expectedNum == actualNum
	}

	return false
}

// toFloat64 converts various numeric types to float64
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// validateCypher performs basic safety checks on Cypher queries
func validateCypher(cypher string) error {
	upper := strings.ToUpper(cypher)

	// Block write operations
	writeKeywords := []string{
		"CREATE", "DELETE", "SET", "REMOVE", "MERGE",
		"DETACH", "DROP", "CALL",
	}
	for _, kw := range writeKeywords {
		if strings.Contains(upper, kw) {
			return &CypherValidationError{
				Message: "Cypher query contains forbidden keyword: " + kw,
			}
		}
	}

	// Must be a read query
	if !strings.Contains(upper, "MATCH") && !strings.Contains(upper, "RETURN") {
		return &CypherValidationError{
			Message: "Cypher query must contain MATCH and RETURN",
		}
	}

	return nil
}

// CypherValidationError indicates a Cypher query failed validation
type CypherValidationError struct {
	Message string
}

func (e *CypherValidationError) Error() string {
	return e.Message
}
