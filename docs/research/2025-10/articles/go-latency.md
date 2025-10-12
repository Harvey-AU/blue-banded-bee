https://medium.com/@yashbatra11111/we-slashed-our-go-apps-latency-by-80-the-trick-was-wild-f9acba8ed3b8

We Slashed Our Go App’s Latency by 80% — The Trick Was Wild Yash Batra Yash
Batra

Follow 6 min read · Apr 28, 2025 455

9

In the fast-paced world of software development, performance is king. Users
expect applications to respond in the blink of an eye, and even a slight delay
can mean the difference between a loyal customer and a lost opportunity. At our
startup, we faced a daunting challenge: our Go-based application, a real-time
data processing platform, was struggling with latency issues that threatened to
derail our growth. After months of trial and error, we uncovered a wild,
unconventional trick that slashed our app’s latency by an astonishing 80%. This
is the story of how we did it, the lessons we learned, and the surprising
technique that turned our performance woes into a triumph.

Press enter or click to view image in full size

The Problem: A Latency Nightmare Our application, built in Go for its
concurrency model and performance benefits, was designed to process massive
streams of data in real time. Think IoT sensor data, financial transactions, and
live analytics — all requiring sub-second response times. Initially, our system
performed admirably, handling thousands of requests per second with ease. But as
our user base grew, so did the strain on our infrastructure.

By early 2024, we noticed a troubling trend: latency was creeping up. What once
took 50 milliseconds to process was now hovering around 250 milliseconds. For
our users, this meant sluggish dashboards, delayed alerts, and a less-than-ideal
experience. Our team dove into the problem, expecting to find a straightforward
fix. Little did we know, we were about to embark on a months-long journey
through profiling, optimization, and a few head-scratching discoveries.

Early Attempts: The Usual Suspects Like any seasoned engineering team, we
started with the basics. We profiled our application using Go’s built-in tools
like pprof and traced bottlenecks with distributed tracing systems. Our initial
findings pointed to a few culprits:

Database Queries: Our PostgreSQL database was handling a high volume of writes,
and some queries were taking longer than expected. Network Overhead: Our
microservices architecture, while modular, introduced latency due to
inter-service communication over gRPC. Goroutine Contention: Go’s lightweight
threads (goroutines) were being overused in some parts of the codebase, leading
to scheduling delays. We tackled these issues systematically. We optimized our
database indexes, introduced connection pooling, and rewrote slow queries. We
fine-tuned our gRPC settings, enabling compression and reducing payload sizes.
We even refactored parts of the codebase to reduce goroutine creation and
improve concurrency patterns. These changes shaved off some latency — bringing
us down to around 180 milliseconds — but we were still far from our target of
sub-50ms responses.

The incremental improvements were encouraging, but we knew we were missing
something big. Our profiling data showed that a significant portion of the
latency was unaccounted for, lurking in what we called the “mystery zone” — a
black box of delays that defied explanation.

The Breakthrough: A Wild Hypothesis One late-night debugging session, fueled by
coffee and desperation, our lead engineer, Sarah, had a wild idea. She noticed
that our application’s CPU usage spiked during peak loads, even though our
profiling data didn’t point to any obvious bottlenecks in the code. “What if,”
she mused, “the issue isn’t in our code at all? What if it’s in how the Go
runtime is interacting with the operating system?”

This was a bold hypothesis. Go is renowned for its efficient runtime, with a
sophisticated garbage collector and scheduler that abstracts away much of the
low-level complexity. But Sarah’s intuition led us to dig deeper into the Go
runtime’s behavior, particularly its interaction with the Linux kernel on our
production servers.

Diving into the Go Runtime We started by analyzing the Go runtime’s scheduling
and garbage collection (GC) behavior. Go’s garbage collector is designed to be
low-latency, but it can still introduce pauses, especially in memory-intensive
applications like ours. We tweaked the GOGC environment variable, which controls
the garbage collection frequency, and saw minor improvements. But the real
breakthrough came when we examined the Go scheduler’s interaction with the Linux
kernel’s Completely Fair Scheduler (CFS).

In Linux, the CFS is responsible for allocating CPU time to processes and
threads. Go’s goroutines are multiplexed onto OS threads by the Go runtime, and
under heavy load, we suspected that the CFS was not prioritizing our
application’s threads effectively. This led us to explore an obscure Linux
feature: cgroup-based CPU bandwidth control.

The Wild Trick: Cgroup Magic Cgroups (control groups) are a Linux kernel feature
that allows you to allocate resources — like CPU, memory, and I/O — to specific
processes. While cgroups are commonly used in containerized environments like
Docker, they’re less frequently employed in bare-metal or VM-based deployments
like ours. However, Sarah’s research revealed that cgroups could be used to
fine-tune CPU allocation at a granular level, potentially reducing contention
between our application’s threads and other system processes.

Here’s what we did:

Isolated Our Application in a Cgroup: We created a dedicated cgroup for our Go
application, ensuring it had a guaranteed share of CPU resources. This prevented
other systemprocesses (like logging daemons or monitoring agents) from stealing
CPU cycles. Tweaked CPU Shares and Quotas: We configured the cgroup to allocate
a higher CPU share to our application during peak loads, using the cpu.shares
and cpu.cfs_quota_us parameters. This ensured that our Go runtime’s threads were
prioritized by the Linux scheduler. Disabled CFS Bandwidth Throttling: By
default, the CFS can throttle processes to enforce fairness. We disabled this
for our cgroup, allowing our application to fully utilize available CPU cores
without artificial delays. The results were staggering. After deploying these
changes to a staging environment, we saw latency drop from 180 milliseconds to
35 milliseconds — an 80% reduction. The “mystery zone” in our profiling data
vanished, as the Go runtime was now able to schedule goroutines with minimal
interference from the kernel.

Why Did This Work? To understand why this cgroup trick was so effective, let’s
break it down:

Reduced Scheduling Latency: The Go scheduler relies on the OS to allocate CPU
time to its threads. By giving our application a dedicated cgroup with higher
priority, we minimized delays caused by the Linux CFS, allowing goroutines to
execute more predictably. Improved Garbage Collection: The garbage collector,
which runs concurrently with the application, benefited from consistent CPU
availability. This reduced GC pauses, further lowering latency. Eliminated
Contention: Our production servers were running multiple processes, including
monitoring tools and background jobs. By isolating our application in a cgroup,
we eliminated resource contention, ensuring that our app had the CPU it needed
when it needed it. This approach was unconventional because it bypassed
traditional application-level optimizations and went straight to the OS layer.
Most Go developers don’t think about cgroups when optimizing their apps, but in
our case, it was the missing piece of the puzzle.

Implementing the Solution For those curious about the technical details, here’s
a simplified version of how we configured the cgroup for our application. This
assumes a Linux system with systemd and cgroup v2.

# Create a cgroup for the application

sudo mkdir /sys/fs/cgroup/myapp

# Set CPU shares (higher value = higher priority)

echo 2048 > /sys/fs/cgroup/myapp/cpu.shares

# Set CPU quota (optional, adjust based on your needs)

echo 500000 > /sys/fs/cgroup/myapp/cpu.cfs_quota_us echo 100000 >
/sys/fs/cgroup/myapp/cpu.cfs_period_us

# Move the application process to the cgroup

echo <pid> > /sys/fs/cgroup/myapp/cgroup.procs In our production environment, we
automated this setup using systemd slices, which provide a more robust way to
manage cgroups. We also integrated cgroup metrics into our monitoring stack to
track CPU allocation and ensure our configuration was working as expected.

Create cgroup for the application sudo mkdir /sys/fs/cgroup/myapp

Configure CPU shares and quotas echo 2048 > /sys/fs/cgroup/myapp/cpu.shares echo
500000 > /sys/fs/cgroup/myapp/cpu.cfs_quota_us echo 100000 >
/sys/fs/cgroup/myapp/cpu.cfs_period_us

Move application process to cgroup (replace with actual PID) echo >
/sys/fs/cgroup/myapp/cgroup.procs
