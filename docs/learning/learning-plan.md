I'm using this project to learn Golang, and this is my learning plan and [concepts](./concepts.md) includes key details I've learnt and want to refer back to.

## Teaching Approach (always)

- Use plain English; define any jargon immediately
- No metaphors or analogies
- Explain one concept at a time with a 3–5 line code snippet from this repo, or two if you can find another simple example
- Also include a common simple use case to help explain it
- Explain how / why the concept is relevant.
- After each snippet, list 2–3 concise bullet summaries of key points
- Ask for a yes/no confirmation before proceeding
- Mark the roadmap item ✓ when the topic is confirmed complete

### Background

I've been working in CSS, HTML, JS & Python for many years, but am just a hack. I've never really learnt properly from anyone or understand the theories or principles of these languages. Jargon and wording really confuse me, so I like to focus on the practical application of things. This project is heavily built with AI, so don't assume I understand all of the code/concepts. My intent is to learn every aspect of the use of Go in this project, so I can stop relying on AI.

## Chapters

### 1. Project & Environment Setup

- [✓] 1.1 What Go is
- [✓] 1.2 Go modules (go.mod & go.sum)
- [✓] 1.3 Directory layout (cmd/, internal/, docs/)

### 2. Main Program Walk‑through

- [✓] 2.1 package main
- [✓] 2.2 import statements
- [✓] 2.3 func main() entry point
- [✓] 2.4 Command‑line flags (using the flag package)

### 3. Variables, Data Types & Control Flow

- [✓] 3.1 Basic types (string, int, bool)
- [✓] 3.2 Composite types (slice, map)
- [✓] 3.3 if statements
- [✓] 3.4 for loops
- [✓] 3.5 switch statements

### 4. Functions & Error Handling

- [✓] 4.1 Declaring functions
- [✓] 4.2 Calling functions
- [✓] 4.3 Multiple return values
- [✓] 4.4 The error type & its handling

### 5. Structs, Methods & Interfaces

- [✓] 5.1 Defining structs
- [✓] 5.2 Receiver methods
- [ ] 5.3 Interfaces
- [ ] 5.4 Polymorphism via interfaces

### 6. Packages & Code Organisation

- [ ] 6.1 Folder-to-package mapping
- [ ] 6.2 Exported vs unexported names
- [ ] 6.3 Import paths
- [ ] 6.4 internal/ vs public packages

### 7. Database Integration (PostgreSQL)

- [ ] 7.1 Opening database connection
- [ ] 7.2 Running queries & scanning results
- [ ] 7.3 Transactions
- [ ] 7.4 Schema migrations

### 8. Concurrency (goroutines & channels)

- [ ] 8.1 Goroutines
- [ ] 8.2 Channels
- [ ] 8.3 select statements
- [ ] 8.4 sync primitives (Mutex, WaitGroup)

### 9. Configuration & Logging

- [ ] 9.1 Reading environment & config files
- [ ] 9.2 Using flag for CLI config
- [ ] 9.3 Structured logging (zerolog)

### 10. Building, Testing & Deployment

- [ ] 10.1 go build & go run
- [ ] 10.2 go test & writing tests
- [ ] 10.3 Packaging & deployment
