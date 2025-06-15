package ulid

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateWithPrefix(t *testing.T) {
	// Test tool ID generation
	toolID := ToolID()

	if !strings.HasPrefix(toolID, "tool-") {
		t.Errorf("Expected tool ID to have 'tool-' prefix, got: %s", toolID)
	}

	// Test that it's a valid ULID
	if !Validate(toolID) {
		t.Errorf("Generated tool ID is not valid: %s", toolID)
	}
}

func TestParseWithPrefix(t *testing.T) {
	// Generate a tool ID
	original := ToolID()

	// Parse it back
	parsed, err := Parse(original)
	if err != nil {
		t.Errorf("Failed to parse tool ID: %v", err)
	}

	// Check that string representation matches
	if parsed.String() != original {
		t.Errorf("Parsed ULID string doesn't match original. Expected: %s, Got: %s", original, parsed.String())
	}

	// Check prefix
	if parsed.Prefix() != PrefixTool {
		t.Errorf("Expected prefix '%s', got '%s'", PrefixTool, parsed.Prefix())
	}
}

func TestUniqueIDs(t *testing.T) {
	// Generate multiple tool IDs and ensure they're unique
	ids := make(map[string]bool)

	for i := 0; i < 100; i++ {
		id := ToolID()
		if ids[id] {
			t.Errorf("Generated duplicate ID: %s", id)
		}
		ids[id] = true
	}
}

func TestTimeOrdering(t *testing.T) {
	// Generate two IDs with a small time gap
	id1 := ToolID()
	time.Sleep(1 * time.Millisecond)
	id2 := ToolID()

	// Parse them
	ulid1, _ := Parse(id1)
	ulid2, _ := Parse(id2)

	// Check that the second one is lexicographically greater
	if ulid1.Compare(ulid2) >= 0 {
		t.Errorf("Expected %s < %s but got %d", id1, id2, ulid1.Compare(ulid2))
	}
}
