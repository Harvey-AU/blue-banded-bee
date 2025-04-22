package jobs

import (
	"time"
)

// JobStatus represents the current status of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusPaused    JobStatus = "paused"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// TaskStatus represents the current status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusSkipped   TaskStatus = "skipped"
)

// Maximum time a task can be "in progress" before being considered stale
const (
	TaskStaleTimeout = 3 * time.Minute
	MaxTaskRetries   = 5
)

// Job represents a crawling job for a domain
type Job struct {
	ID              string    `json:"id"`
	Domain          string    `json:"domain"`
	Status          JobStatus `json:"status"`
	Progress        float64   `json:"progress"`
	TotalTasks      int       `json:"total_tasks"`
	CompletedTasks  int       `json:"completed_tasks"`
	FailedTasks     int       `json:"failed_tasks"`
	CreatedAt       time.Time `json:"created_at"`
	StartedAt       time.Time `json:"started_at,omitempty"`
	CompletedAt     time.Time `json:"completed_at,omitempty"`
	Concurrency     int       `json:"concurrency"`
	FindLinks       bool      `json:"find_links"`
	MaxDepth        int       `json:"max_depth"`
	IncludePaths    []string  `json:"include_paths,omitempty"`
	ExcludePaths    []string  `json:"exclude_paths,omitempty"`
	RequiredWorkers int       `json:"required_workers"`
	ErrorMessage    string    `json:"error_message,omitempty"`
}

// Task represents a single URL to be crawled within a job
type Task struct {
	ID          string     `json:"id"`
	JobID       string     `json:"job_id"`
	PageID      int        `json:"page_id"`
	Path        string     `json:"path"`
	Status      TaskStatus `json:"status"`
	Depth       int        `json:"depth"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   time.Time  `json:"started_at,omitempty"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
	RetryCount  int        `json:"retry_count"`
	Error       string     `json:"error,omitempty"`

	// Source information
	SourceType string `json:"source_type"`          // "sitemap", "link", "manual"
	SourceURL  string `json:"source_url,omitempty"` // URL where this was discovered (for links)

	// Result data
	StatusCode   int    `json:"status_code,omitempty"`
	ResponseTime int64  `json:"response_time,omitempty"`
	CacheStatus  string `json:"cache_status,omitempty"`
	ContentType  string `json:"content_type,omitempty"`
	
	// Job configuration that affects processing
	FindLinks bool `json:"-"` // Not stored in DB, just used during processing
}

// JobOptions defines configuration options for a crawl job
type JobOptions struct {
	Domain          string   `json:"domain"`
	StartURLs       []string `json:"start_urls,omitempty"`
	UseSitemap      bool     `json:"use_sitemap"`
	Concurrency     int      `json:"concurrency"`
	FindLinks       bool     `json:"find_links"`
	MaxDepth        int      `json:"max_depth"`
	IncludePaths    []string `json:"include_paths,omitempty"`
	ExcludePaths    []string `json:"exclude_paths,omitempty"`
	RequiredWorkers int      `json:"required_workers"`
}

// Create a separate CrawlResult struct for batch operations
type CrawlResultData struct {
	JobID        string
	TaskID       string
	URL          string
	ResponseTime int64
	StatusCode   int
	Error        string
	CacheStatus  string
	ContentType  string
}
