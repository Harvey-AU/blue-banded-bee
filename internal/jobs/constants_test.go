package jobs

import (
	"testing"
)

func TestStatusConstants(t *testing.T) {
	// Test JobStatus constants
	jobStatusTests := map[JobStatus]string{
		JobStatusPending:   "pending",
		JobStatusRunning:   "running",
		JobStatusPaused:    "paused",
		JobStatusCompleted: "completed",
		JobStatusFailed:    "failed",
		JobStatusCancelled: "cancelled",
	}

	for status, expected := range jobStatusTests {
		if string(status) != expected {
			t.Errorf("JobStatus %v: expected '%s', got '%s'",
				status, expected, string(status))
		}
	}

	// Test TaskStatus constants
	taskStatusTests := map[TaskStatus]string{
		TaskStatusPending:   "pending",
		TaskStatusRunning:   "running",
		TaskStatusCompleted: "completed",
		TaskStatusFailed:    "failed",
		TaskStatusSkipped:   "skipped",
	}

	for status, expected := range taskStatusTests {
		if string(status) != expected {
			t.Errorf("TaskStatus %v: expected '%s', got '%s'",
				status, expected, string(status))
		}
	}
}
