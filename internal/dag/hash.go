// Package dag implements the DAG-based migration graph — migratex's
// core differentiator over linear migration tools.
//
// Instead of 001, 002, 003... migrations are nodes in a directed acyclic graph,
// like git commits. This enables safe branch merging and conflict detection.
package dag

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

// ComputeID generates a deterministic migration ID from parents and operations.
// It's content-addressable like git: same inputs always produce the same hash.
// We truncate to 12 hex chars for human readability.
func ComputeID(parents []string, operations any) string {
	// Sort parents for deterministic hashing regardless of order
	sorted := make([]string, len(parents))
	copy(sorted, parents)
	sort.Strings(sorted)

	content := struct {
		Parents    []string `json:"parents"`
		Operations any      `json:"operations"`
	}{
		Parents:    sorted,
		Operations: operations,
	}

	// json.Marshal converts Go structs to JSON bytes.
	// The _ ignores the error (safe here since we control the input types).
	data, _ := json.Marshal(content)

	hash := sha256.Sum256(data)
	// fmt.Sprintf with %x formats bytes as hexadecimal
	return fmt.Sprintf("%x", hash[:6]) // 6 bytes = 12 hex chars
}

// ComputeChecksum generates a SHA-256 hash of SQL content for tamper detection.
func ComputeChecksum(sql string) string {
	hash := sha256.Sum256([]byte(sql))
	return fmt.Sprintf("%x", hash)
}
