https://medium.com/@cleanCompile/10-golang-performance-tips-you-wont-find-in-the-docs-6559665469da

10 Golang Performance Tips You Won’t Find in the Docs Clean Compiler Clean
Compiler

Follow 2 min read · Sep 11, 2025 6

Golang (Go) is already fast by design — compiled, lightweight, and
concurrency-friendly. But in real-world production systems, performance
bottlenecks creep in where you least expect them.

Over the years, I’ve collected some hard-earned performance tips that you won’t
usually find in official docs. Here are 10 that can make your Go apps noticeably
faster. Press enter or click to view image in full size

1. Preallocate Slices Instead of Growing Them data := make([]int, 0, 1000) //
   preallocate Growing slices dynamically forces Go to reallocate memory
   repeatedly. If you know the approximate size, always preallocate.

2. Reuse Objects with sync.Pool Creating objects in hot paths is expensive. Use
   sync.Pool to recycle them and reduce GC pressure.

var bufPool = sync.Pool{ New: func() interface{} { return new(bytes.Buffer) },
} 3. Minimize Goroutines Yes, Go makes spawning goroutines cheap — but millions
of them will still overwhelm your scheduler.

Pool workers instead of spawning unbounded goroutines.

4. Use Buffered Channels Unbuffered channels block more often than you think.
   Even a small buffer (like size 1 or 10) can dramatically reduce contention.

5. Avoid Interface{} in Hot Paths Using interface{} forces allocations and type
   assertions. If you can, use generics or concrete types for critical sections.

6. Prefer sync.RWMutex Over sync.Mutex If reads dominate writes, RWMutex allows
   multiple readers without blocking. Don’t just reach for Mutex blindly.

7. Reduce JSON Overhead Go’s default encoding/json is convenient but slow. For
   high-performance APIs:

Use jsoniter or easyjson. Predefine structs instead of
map[string]interface{}. 8. Minimize String Conversions Constantly converting
between []byte and string allocates memory. Cache conversions or avoid them when
possible.

9. Profile Before Optimizing I wasted hours tweaking “slow” code that wasn’t the
   real problem. Always use:

go test -bench . go tool pprof Let the profiler guide you.

10. Use Build Tags for Specialized Optimizations Go build tags (// +build) let
    you maintain optimized versions of code for specific platforms or workloads.
    Perfect for performance-critical systems.

Final Thoughts Go’s simplicity hides a lot of performance pitfalls. By applying
these 10 practices — from preallocating slices to profiling with pprof — you’ll
squeeze out more efficiency without complicating your codebase.
