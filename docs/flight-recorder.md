# Flight Recorder

This document explains how to use the flight recorder for performance debugging.

## Enabling the Flight Recorder

The flight recorder can be enabled by setting the `FLIGHT_RECORDER_ENABLED` environment variable to `true`. When enabled, the application will write a trace file named `trace.out` to the root of the project.

## Accessing the Trace Data

The trace data can be accessed via the `/debug/fgtrace` endpoint. This endpoint will return the `trace.out` file, which can be analyzed using `go tool trace`.

### Example Usage

1.  **Start the application with the flight recorder enabled:**
    ```bash
    FLIGHT_RECORDER_ENABLED=true go run cmd/app/main.go
    ```

2.  **Access the trace data:**
    ```bash
    curl -o trace.out http://localhost:8080/debug/fgtrace
    ```

3.  **Analyze the trace data:**
    ```bash
    go tool trace trace.out
    ```
