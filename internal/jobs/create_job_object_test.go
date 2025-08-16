package jobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateJobObject(t *testing.T) {
	tests := []struct {
		name             string
		options          *JobOptions
		normalisedDomain string
		validateFunc     func(*testing.T, *Job, *JobOptions, string)
	}{
		{
			name: "minimal_options",
			options: &JobOptions{
				Domain:      "example.com",
				Concurrency: 5,
				FindLinks:   true,
				MaxPages:    0,
			},
			normalisedDomain: "example.com",
			validateFunc: func(t *testing.T, job *Job, opts *JobOptions, domain string) {
				assert.Equal(t, domain, job.Domain)
				assert.Equal(t, JobStatusPending, job.Status)
				assert.Equal(t, 0.0, job.Progress)
				assert.Equal(t, 0, job.TotalTasks)
				assert.Equal(t, opts.Concurrency, job.Concurrency)
				assert.Equal(t, opts.FindLinks, job.FindLinks)
				assert.Equal(t, opts.MaxPages, job.MaxPages)
			},
		},
		{
			name: "full_options",
			options: &JobOptions{
				Domain:          "test.com",
				UserID:          func() *string { s := "user-123"; return &s }(),
				OrganisationID:  func() *string { s := "org-456"; return &s }(),
				Concurrency:     10,
				FindLinks:       false,
				MaxPages:        100,
				IncludePaths:    []string{"/api/*", "/docs/*"},
				ExcludePaths:    []string{"/admin/*"},
				RequiredWorkers: 5,
				SourceType:      func() *string { s := "api"; return &s }(),
				SourceDetail:    func() *string { s := "dashboard"; return &s }(),
				SourceInfo:      func() *string { s := `{"version": "1.0"}`; return &s }(),
			},
			normalisedDomain: "test.com",
			validateFunc: func(t *testing.T, job *Job, opts *JobOptions, domain string) {
				assert.Equal(t, domain, job.Domain)
				assert.Equal(t, opts.UserID, job.UserID)
				assert.Equal(t, opts.OrganisationID, job.OrganisationID)
				assert.Equal(t, opts.Concurrency, job.Concurrency)
				assert.Equal(t, opts.FindLinks, job.FindLinks)
				assert.Equal(t, opts.MaxPages, job.MaxPages)
				assert.Equal(t, opts.IncludePaths, job.IncludePaths)
				assert.Equal(t, opts.ExcludePaths, job.ExcludePaths)
				assert.Equal(t, opts.RequiredWorkers, job.RequiredWorkers)
				assert.Equal(t, opts.SourceType, job.SourceType)
				assert.Equal(t, opts.SourceDetail, job.SourceDetail)
				assert.Equal(t, opts.SourceInfo, job.SourceInfo)
			},
		},
		{
			name: "domain_normalisation_applied",
			options: &JobOptions{
				Domain:      "EXAMPLE.COM", // Original domain
				Concurrency: 1,
			},
			normalisedDomain: "example.com", // Normalised domain
			validateFunc: func(t *testing.T, job *Job, opts *JobOptions, domain string) {
				assert.Equal(t, domain, job.Domain, "Should use normalised domain, not original")
				assert.NotEqual(t, opts.Domain, job.Domain, "Should not use original domain")
			},
		},
		{
			name: "pointer_fields_preserved",
			options: &JobOptions{
				Domain:         "test.com",
				UserID:         func() *string { s := "user-abc"; return &s }(),
				OrganisationID: func() *string { s := "org-xyz"; return &s }(),
				SourceType:     func() *string { s := "webhook"; return &s }(),
			},
			normalisedDomain: "test.com",
			validateFunc: func(t *testing.T, job *Job, opts *JobOptions, domain string) {
				// Test that pointer fields are properly assigned
				require.NotNil(t, job.UserID)
				assert.Equal(t, *opts.UserID, *job.UserID)
				
				require.NotNil(t, job.OrganisationID)
				assert.Equal(t, *opts.OrganisationID, *job.OrganisationID)
				
				require.NotNil(t, job.SourceType)
				assert.Equal(t, *opts.SourceType, *job.SourceType)
			},
		},
		{
			name: "nil_pointer_fields_preserved",
			options: &JobOptions{
				Domain:         "test.com",
				UserID:         nil,
				OrganisationID: nil,
				SourceType:     nil,
				SourceDetail:   nil,
				SourceInfo:     nil,
			},
			normalisedDomain: "test.com",
			validateFunc: func(t *testing.T, job *Job, opts *JobOptions, domain string) {
				assert.Nil(t, job.UserID)
				assert.Nil(t, job.OrganisationID)
				assert.Nil(t, job.SourceType)
				assert.Nil(t, job.SourceDetail)
				assert.Nil(t, job.SourceInfo)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record time before creation to test CreatedAt
			beforeTime := time.Now().UTC()
			
			job := createJobObject(tt.options, tt.normalisedDomain)
			
			afterTime := time.Now().UTC()

			// Verify common fields
			assert.NotEmpty(t, job.ID, "ID should be generated")
			assert.Equal(t, JobStatusPending, job.Status)
			assert.Equal(t, 0.0, job.Progress)
			assert.Equal(t, 0, job.TotalTasks)
			assert.Equal(t, 0, job.CompletedTasks)
			assert.Equal(t, 0, job.FoundTasks)
			assert.Equal(t, 0, job.SitemapTasks)
			assert.Equal(t, 0, job.FailedTasks)

			// Verify CreatedAt is within reasonable time range
			assert.True(t, job.CreatedAt.After(beforeTime) || job.CreatedAt.Equal(beforeTime))
			assert.True(t, job.CreatedAt.Before(afterTime) || job.CreatedAt.Equal(afterTime))

			// Run custom validation for this test case
			if tt.validateFunc != nil {
				tt.validateFunc(t, job, tt.options, tt.normalisedDomain)
			}
		})
	}
}

func TestCreateJobObjectDefaults(t *testing.T) {
	// Test that default values are correctly set
	options := &JobOptions{
		Domain: "example.com",
		// All other fields left as zero values
	}
	
	job := createJobObject(options, "example.com")
	
	// Verify all counters start at zero
	assert.Equal(t, 0, job.TotalTasks)
	assert.Equal(t, 0, job.CompletedTasks)
	assert.Equal(t, 0, job.FoundTasks)
	assert.Equal(t, 0, job.SitemapTasks)
	assert.Equal(t, 0, job.FailedTasks)
	assert.Equal(t, 0.0, job.Progress)
	
	// Verify status is pending
	assert.Equal(t, JobStatusPending, job.Status)
	
	// Verify ID is generated (UUID format)
	assert.Len(t, job.ID, 36) // UUID length with hyphens
	assert.Contains(t, job.ID, "-")
}

func TestCreateJobObjectUniqueIDs(t *testing.T) {
	// Test that each job gets a unique ID
	options := &JobOptions{Domain: "example.com"}
	
	job1 := createJobObject(options, "example.com")
	job2 := createJobObject(options, "example.com")
	
	assert.NotEqual(t, job1.ID, job2.ID, "Each job should get a unique ID")
	assert.NotEmpty(t, job1.ID)
	assert.NotEmpty(t, job2.ID)
}

func TestCreateJobObjectTimeHandling(t *testing.T) {
	// Test that CreatedAt is properly set and increases over time
	options := &JobOptions{Domain: "example.com"}
	
	job1 := createJobObject(options, "example.com")
	time.Sleep(1 * time.Millisecond) // Small delay
	job2 := createJobObject(options, "example.com")
	
	assert.True(t, job2.CreatedAt.After(job1.CreatedAt) || job2.CreatedAt.Equal(job1.CreatedAt))
	
	// Verify both times are in UTC
	assert.Equal(t, time.UTC, job1.CreatedAt.Location())
	assert.Equal(t, time.UTC, job2.CreatedAt.Location())
}