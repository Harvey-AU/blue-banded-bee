package api

// This file has been refactored into separate, focused test files:
// - test_mocks.go: Common mocks and test utilities
// - jobs_create_test.go: POST /v1/jobs endpoint tests
// - dashboard_endpoints_test.go: Dashboard stats and activity tests  
// - webhook_endpoints_test.go: Webflow webhook integration tests
//
// This separation follows the Extract + Test + Commit methodology
// applied to test file organization for better maintainability.
//
// TODO: This file can be removed once all tests are confirmed working
// in their new separated locations.