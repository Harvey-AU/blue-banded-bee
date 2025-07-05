# Content Storage & Change Tracking Implementation Plan

This document outlines the strategy for tracking changes in crawled web pages and storing their historical HTML content efficiently.

## 1. Core Objectives

- **Reliable Change Detection:** Accurately determine if the meaningful content of a page has changed between crawls, ignoring irrelevant noise like cache-busting hashes in script names, dynamic class names, or comments.
- **Efficient Storage:** Store the full HTML content of a page *only* when it has meaningfully changed.
- **Scalable Architecture:** Ensure the solution is performant and cost-effective at scale, with a clear path for long-term data archiving.

## 2. Implementation Strategy: Semantic Hashing

To achieve reliable change detection, we will use a **Semantic Hashing** approach instead of a naive hash of the raw HTML. This process is designed to create a "fingerprint" of the page's meaningful content.

The workflow for each crawled page will be:

1.  **Parse HTML:** Use a standard Go HTML parser (e.g., `golang.org/x/net/html`) to convert the raw HTML into a structured DOM tree. This is crucial for understanding the page's structure.
2.  **Extract Canonical Content:** Traverse the DOM tree and build a clean, canonical string representation of the page. This involves:
    *   **Discarding Noise:** Explicitly removing entire nodes like `<head>`, `<script>`, `<style>`, and HTML comments.
    *   **Extracting Text:** Collecting the text content from meaningful tags within the `<body>` (e.g., `h1`, `p`, `div`, `a`, etc.).
    *   **Selective Attribute Inclusion:** To capture significant framework-level changes (e.g., a new Webflow page ID), we will maintain a **whitelist** of important attributes (e.g., `data-wf-page`). If a whitelisted attribute is found, its name and value will be included in the canonical string. All other attributes will be ignored.
3.  **Generate Hash:** Calculate a SHA-256 hash of the final canonical string. This hash represents the page's meaningful state.

This process ensures that only significant changes to user-visible content or whitelisted attributes will result in a new hash. The performance overhead of this local processing is negligible (<10ms) compared to network latency.

## 3. Database Schema Modifications

To support this system, the following columns will be added:

-   **`tasks` table:**
    -   `content_hash TEXT`: Stores the semantic hash generated for this specific crawl task.
    -   `html_storage_path TEXT NULL`: Stores the path to the archived HTML file in object storage (e.g., Supabase Storage). This will be `NULL` if the content has not changed since the last crawl.
-   **`pages` table:**
    -   `latest_content_hash TEXT NULL`: Caches the most recent hash for each unique page. This is used for efficient comparison to determine if a page has changed, avoiding a slow search through the `tasks` table history.

## 4. Storage Workflow

1.  After generating the semantic hash for a crawled page, the worker will compare it to the `latest_content_hash` stored in the `pages` table.
2.  **If the hashes are different:**
    a. The full HTML content will be uploaded to **Supabase Storage**.
    b. The `tasks` record will be updated with the new `content_hash` and the `html_storage_path`.
    c. The `pages` record will be updated with the new `latest_content_hash`.
3.  **If the hashes are the same:**
    a. No HTML will be stored.
    b. The `tasks` record will be updated with the (unchanged) `content_hash`, and `html_storage_path` will remain `NULL`.

This approach provides a complete history of when content changed via the hashes, while only incurring storage costs for the changed versions.
