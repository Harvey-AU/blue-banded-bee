# Implementation Plan: Increase Test Coverage

**Objective:** Significantly increase the test coverage for the `internal/jobs` package from its current low percentage to provide a robust safety net for future development.

This plan focuses on creating meaningful integration tests that verify the interaction between the `JobManager`, `WorkerPool`, and the PostgreSQL database.

---

## Overall Approach & Complexity

-   **Complexity:** Medium to High. The code is concurrent (worker pools, goroutines for sitemap processing) and heavily stateful (all logic depends on the PostgreSQL database). Tests must manage this state carefully.
-   **Method:** We will use a real test database. For each test, the workflow will be:
    1.  Set up the required state in the database (e.g., create a user, create a pending job).
    2.  Execute the function being tested.
    3.  Query the database to assert that the state has changed as expected (e.g., the job status is now `running`).
-   **Dependencies:** The `crawler` component will be mocked to prevent real network requests during tests.

---

## Test Breakdown

### 1. `manager_test.go`

| Test Function | Est. Lines of Code | Complexity | Notes |
| :--- | :--- | :--- | :--- |
| **`TestCreateJob`** | 40-60 lines | **Medium** | Requires DB setup to create a job and then query to verify its properties. Testing the "cancel existing" logic adds a bit more setup. |
| **`TestCreateJob_Sitemap`** | 50-70 lines | **High** | This test is complex because `processSitemap` runs in a goroutine. It will require mocking the crawler and using techniques (like polling the DB) to wait for the concurrent process to finish before asserting the results. |
| **`TestCancelJob`** | 30-40 lines | **Medium** | Requires creating a job in a `running` state first, then calling cancel and asserting the new `cancelled` state in the DB. |
| **`TestGetJob`** | 25-35 lines | **Low** | The simplest test. Create a job, fetch it, and compare the results. |
| **`TestEnqueueJobURLs`** | 40-50 lines | **Medium** | Needs to test the in-memory duplicate check (`processedPages` map) along with the database interaction, ensuring duplicates are not written to the `tasks` table. |

### 2. `worker_test.go`

| Test Function | Est. Lines of Code | Complexity | Notes |
| :--- | :--- | :--- | :--- |
| **`TestWorkerPool_StartStop`**| 20-30 lines | **Low** | A simple sanity check to ensure the pool starts and stops without deadlocking. |
| **`TestWorkerPool_ProcessJob`**| 60-80 lines | **Very High** | This is the most complex test. It's a full end-to-end integration test that involves the `JobManager`, `WorkerPool`, a mocked `Crawler`, and the database. It will verify the entire core loop. |
| **`TestProcessTask`** | 40-50 lines | **Medium** | This will test the `processTask` function in isolation, which is simpler than testing the whole pool. It still requires a mocked crawler and DB assertions. |
| **`TestIsSameOrSubDomain`** | 40-50 lines | **Low** | This is a pure helper function. It will be a simple, table-driven unit test with no external dependencies. |

---

## Summary

-   **Total Estimated Lines of Code:** 350 - 470 lines
-   **Total Estimated Time:** Approximately 1 to 1.5 hours.

This plan outlines the necessary steps to build a comprehensive test suite for the core job processing logic.
