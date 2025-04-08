package crawler

import "time"

type Config struct {
    DefaultTimeout    time.Duration
    MaxConcurrency   int
    RateLimit        int
    UserAgent        string
    RetryAttempts    int
    RetryDelay       time.Duration
    Port             string
    Env              string
    LogLevel         string
    DatabaseURL      string
    AuthToken        string
    SentryDSN        string
}

func DefaultConfig() *Config {
    return &Config{
        DefaultTimeout:  30 * time.Second,
        MaxConcurrency: 5,
        RateLimit:      10,
        UserAgent:      "Cache-Warmer Bot",
        RetryAttempts:  3,
        RetryDelay:     2 * time.Second,
    }
}
