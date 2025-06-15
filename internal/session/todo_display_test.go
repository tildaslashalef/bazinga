package session

import (
	"strings"
	"testing"
)

func TestFormatTodoList(t *testing.T) {
	session := &Session{}

	todos := []TodoItem{
		{ID: "1", Content: "Setup project structure", Status: "completed", Priority: "high"},
		{ID: "2", Content: "Implement authentication", Status: "in_progress", Priority: "high"},
		{ID: "3", Content: "Add user dashboard", Status: "pending", Priority: "medium"},
		{ID: "4", Content: "Write documentation", Status: "pending", Priority: "low"},
	}

	result := session.FormatTodoList(todos)

	// Check that it contains expected elements
	expectedElements := []string{
		"📋 **Task Breakdown:**",
		"[x] ✅ 🔥", // completed high priority
		"[ ] ⏳ 🔥", // in progress high priority
		"[ ] ⭕",   // pending medium priority
		"[ ] ⭕ 💫", // pending low priority
		"**Progress:** 1/4 tasks completed (25%)",
	}

	for _, element := range expectedElements {
		if !strings.Contains(result, element) {
			t.Errorf("Expected result to contain '%s', but it didn't. Result:\n%s", element, result)
		}
	}
}

func TestShowTodoProgress(t *testing.T) {
	session := &Session{}

	todos := []TodoItem{
		{ID: "1", Content: "Task 1", Status: "completed", Priority: "high"},
		{ID: "2", Content: "Task 2", Status: "completed", Priority: "medium"},
		{ID: "3", Content: "Task 3", Status: "in_progress", Priority: "medium"},
		{ID: "4", Content: "Task 4", Status: "pending", Priority: "low"},
	}

	result := session.ShowTodoProgress(todos)

	expectedElements := []string{
		"📊 **Progress Update:**",
		"✅ Completed: 2",
		"⏳ In Progress: 1",
		"⭕ Pending: 1",
		"📈 Overall: 50% complete",
		"**Next:** 💫 Task 4", // next pending low priority task
	}

	for _, element := range expectedElements {
		if !strings.Contains(result, element) {
			t.Errorf("Expected result to contain '%s', but it didn't. Result:\n%s", element, result)
		}
	}
}

func TestShowTodoProgress_AllComplete(t *testing.T) {
	session := &Session{}

	todos := []TodoItem{
		{ID: "1", Content: "Task 1", Status: "completed", Priority: "high"},
		{ID: "2", Content: "Task 2", Status: "completed", Priority: "medium"},
	}

	result := session.ShowTodoProgress(todos)

	if !strings.Contains(result, "✨ **All Tasks Complete!**") {
		t.Errorf("Expected completion message when all tasks are done. Result:\n%s", result)
	}
}

func TestFormatTodoUpdate(t *testing.T) {
	session := &Session{}

	todo := TodoItem{
		ID:       "1",
		Content:  "Implement feature X",
		Status:   "completed",
		Priority: "high",
	}

	result := session.FormatTodoUpdate(todo, "in_progress")

	expectedElements := []string{
		"✅ **Completed:**",
		"🔥", // high priority indicator
		"Implement feature X",
	}

	for _, element := range expectedElements {
		if !strings.Contains(result, element) {
			t.Errorf("Expected result to contain '%s', but it didn't. Result:\n%s", element, result)
		}
	}
}

func TestCreateQuickTodoSummary(t *testing.T) {
	session := &Session{}

	todos := []TodoItem{
		{ID: "1", Content: "Task 1", Status: "completed", Priority: "high"},
		{ID: "2", Content: "Task 2", Status: "pending", Priority: "medium"},
		{ID: "3", Content: "Task 3", Status: "pending", Priority: "low"},
	}

	result := session.CreateQuickTodoSummary(todos)
	expected := "📋 Progress: 1/3 tasks (33%)"

	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestCreateQuickTodoSummary_AllComplete(t *testing.T) {
	session := &Session{}

	todos := []TodoItem{
		{ID: "1", Content: "Task 1", Status: "completed", Priority: "high"},
		{ID: "2", Content: "Task 2", Status: "completed", Priority: "medium"},
	}

	result := session.CreateQuickTodoSummary(todos)
	expected := "✨ All tasks completed!"

	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestGetStatusDisplay(t *testing.T) {
	tests := []struct {
		status           string
		expectedCheckbox string
		expectedIcon     string
	}{
		{"completed", "x", "✅"},
		{"in_progress", " ", "⏳"},
		{"canceled", " ", "❌"},
		{"pending", " ", "⭕"},
		{"unknown", " ", "⭕"}, // defaults to pending
	}

	for _, test := range tests {
		checkbox, icon := getStatusDisplay(test.status)
		if checkbox != test.expectedCheckbox {
			t.Errorf("Status '%s': expected checkbox '%s', got '%s'", test.status, test.expectedCheckbox, checkbox)
		}
		if icon != test.expectedIcon {
			t.Errorf("Status '%s': expected icon '%s', got '%s'", test.status, test.expectedIcon, icon)
		}
	}
}

func TestGetPriorityIndicator(t *testing.T) {
	tests := []struct {
		priority string
		expected string
	}{
		{"high", "🔥 "},
		{"low", "💫 "},
		{"medium", ""},
		{"unknown", ""}, // defaults to medium
	}

	for _, test := range tests {
		result := getPriorityIndicator(test.priority)
		if result != test.expected {
			t.Errorf("Priority '%s': expected '%s', got '%s'", test.priority, test.expected, result)
		}
	}
}

func TestExampleTodoDisplay(t *testing.T) {
	result := ExampleTodoDisplay()

	// Should contain the example formatting
	expectedElements := []string{
		"📋 **Task Breakdown:**",
		"[x] ✅ 🔥 Analyze current theme structure",
		"[ ] ⏳ 🔥 Create dark theme configuration",
		"[ ] ⭕ Add theme toggle component",
		"[ ] ⭕ Update existing components",
		"[ ] ⭕ 💫 Test theme switching",
		"**Progress:** 1/5 tasks completed (20%)",
	}

	for _, element := range expectedElements {
		if !strings.Contains(result, element) {
			t.Errorf("Expected example to contain '%s', but it didn't. Result:\n%s", element, result)
		}
	}
}
