package jobs

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// DomainLimiterConfig controls adaptive throttling behaviour.
type DomainLimiterConfig struct {
	BaseDelay             time.Duration
	DelayStep             time.Duration
	SuccessProbeThreshold int
	MaxAdaptiveDelay      time.Duration
	ConcurrencyStep       time.Duration
	PersistInterval       time.Duration
	MaxBlockingRetries    int
	CancelRateLimitJobs   bool
	CancelStreakThreshold int
	CancelDelayThreshold  time.Duration
}

func defaultDomainLimiterConfig() DomainLimiterConfig {
	cfg := DomainLimiterConfig{
		BaseDelay:             500 * time.Millisecond,
		DelayStep:             time.Second,
		SuccessProbeThreshold: 20,
		MaxAdaptiveDelay:      60 * time.Second,
		ConcurrencyStep:       5 * time.Second,
		PersistInterval:       30 * time.Second,
		MaxBlockingRetries:    3,
		CancelRateLimitJobs:   false,
		CancelStreakThreshold: 20,
		CancelDelayThreshold:  60 * time.Second,
	}

	if v, ok := os.LookupEnv("BBB_RATE_LIMIT_BASE_DELAY_MS"); ok {
		if ms, err := strconv.Atoi(v); err == nil && ms >= 0 {
			cfg.BaseDelay = time.Duration(ms) * time.Millisecond
		}
	}
	if v, ok := os.LookupEnv("BBB_RATE_LIMIT_MAX_DELAY_SECONDS"); ok {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			cfg.MaxAdaptiveDelay = time.Duration(sec) * time.Second
		}
	}
	if v, ok := os.LookupEnv("BBB_RATE_LIMIT_SUCCESS_THRESHOLD"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.SuccessProbeThreshold = n
		}
	}
	if v, ok := os.LookupEnv("BBB_RATE_LIMIT_MAX_RETRIES"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxBlockingRetries = n
		}
	}
	if v, ok := os.LookupEnv("BBB_RATE_LIMIT_CANCEL_THRESHOLD"); ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.CancelStreakThreshold = n
		}
	}
	if v, ok := os.LookupEnv("BBB_RATE_LIMIT_CANCEL_DELAY_SECONDS"); ok {
		if sec, err := strconv.Atoi(v); err == nil && sec >= 0 {
			cfg.CancelDelayThreshold = time.Duration(sec) * time.Second
		}
	}
	if v, ok := os.LookupEnv("BBB_RATE_LIMIT_CANCEL_ENABLED"); ok {
		cfg.CancelRateLimitJobs = v == "1" || v == "true" || v == "TRUE"
	}

	return cfg
}

// DomainLimiter coordinates request pacing across workers for each domain.
type DomainLimiter struct {
	cfg     DomainLimiterConfig
	dbQueue DbQueueInterface

	mu      sync.Mutex
	domains map[string]*domainState

	now func() time.Time
}

// DomainRequest describes a request against a domain that needs throttling.
type DomainRequest struct {
	Domain         string
	JobID          string
	RobotsDelay    time.Duration
	JobConcurrency int
}

// DomainPermit is returned by Acquire and must be released after the request completes.
type DomainPermit struct {
	limiter *DomainLimiter
	domain  string
	jobID   string
	delay   time.Duration
}

func newDomainLimiter(dbQueue DbQueueInterface) *DomainLimiter {
	return &DomainLimiter{
		cfg:     defaultDomainLimiterConfig(),
		dbQueue: dbQueue,
		domains: make(map[string]*domainState),
		now:     time.Now,
	}
}

// Seed initialises limiter state for a domain with persisted values.
func (dl *DomainLimiter) Seed(domain string, baseDelaySeconds int, adaptiveDelaySeconds int, floorSeconds int) {
	state := dl.getOrCreateState(domain)
	state.mu.Lock()
	defer state.mu.Unlock()

	base := time.Duration(baseDelaySeconds) * time.Second
	if base < dl.cfg.BaseDelay {
		base = dl.cfg.BaseDelay
	}
	state.baseDelay = base

	adaptive := time.Duration(adaptiveDelaySeconds) * time.Second
	if adaptive < base {
		adaptive = base
	}
	if adaptive > dl.cfg.MaxAdaptiveDelay {
		adaptive = dl.cfg.MaxAdaptiveDelay
	}
	state.adaptiveDelay = adaptive

	floor := time.Duration(floorSeconds) * time.Second
	if floor < 0 {
		floor = 0
	}
	if floor > adaptive {
		floor = adaptive
	}
	state.delayFloor = floor
}

// Acquire waits until the caller is allowed to perform a request against the domain.
func (dl *DomainLimiter) Acquire(ctx context.Context, req DomainRequest) (*DomainPermit, error) {
	if req.Domain == "" {
		return &DomainPermit{limiter: dl, domain: "", jobID: req.JobID}, nil
	}

	state := dl.getOrCreateState(req.Domain)
	delay, err := state.acquire(ctx, dl.cfg, dl.now, req)
	if err != nil {
		return nil, err
	}

	return &DomainPermit{
		limiter: dl,
		domain:  req.Domain,
		jobID:   req.JobID,
		delay:   delay,
	}, nil
}

// Release notifies the limiter about the outcome of a request.
func (p *DomainPermit) Release(success bool, rateLimited bool) {
	if p == nil || p.limiter == nil || p.domain == "" {
		return
	}
	p.limiter.release(p.domain, p.jobID, success, rateLimited)
}

// UpdateRobotsDelay allows adjusting the base delay when robots.txt changes.
func (dl *DomainLimiter) UpdateRobotsDelay(domain string, delaySeconds int) {
	state := dl.getOrCreateState(domain)
	state.mu.Lock()
	defer state.mu.Unlock()

	base := time.Duration(delaySeconds) * time.Second
	if base < dl.cfg.BaseDelay {
		base = dl.cfg.BaseDelay
	}
	state.baseDelay = base
	if state.adaptiveDelay < base {
		state.adaptiveDelay = base
	}
}

// Domain state ---------------------------------------------------------------------------------

type domainState struct {
	mu   sync.Mutex
	cond *sync.Cond

	baseDelay     time.Duration
	adaptiveDelay time.Duration
	delayFloor    time.Duration

	errorStreak   int
	successStreak int

	nextAvailable time.Time
	backoffUntil  time.Time

	lastPersist time.Time

	probing       bool
	probePrevious time.Duration
	probeTarget   time.Duration

	jobStates map[string]*jobDomainState
}

type jobDomainState struct {
	original int
	allowed  int
	active   int
}

func newDomainState(base time.Duration) *domainState {
	ds := &domainState{
		baseDelay: base,
		jobStates: make(map[string]*jobDomainState),
	}
	ds.cond = sync.NewCond(&ds.mu)
	return ds
}

func (dl *DomainLimiter) getOrCreateState(domain string) *domainState {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if state, ok := dl.domains[domain]; ok {
		return state
	}

	state := newDomainState(dl.cfg.BaseDelay)
	dl.domains[domain] = state
	return state
}

func (ds *domainState) ensureJobState(jobID string, concurrency int) *jobDomainState {
	js, ok := ds.jobStates[jobID]
	if !ok {
		js = &jobDomainState{original: concurrency, allowed: concurrency}
		ds.jobStates[jobID] = js
	}
	js.original = concurrency
	if js.allowed <= 0 {
		js.allowed = concurrency
	}
	return js
}

func (ds *domainState) effectiveDelay(cfg DomainLimiterConfig) time.Duration {
	delay := ds.adaptiveDelay
	if delay < ds.baseDelay {
		delay = ds.baseDelay
	}
	if delay > cfg.MaxAdaptiveDelay {
		delay = cfg.MaxAdaptiveDelay
	}
	return delay
}

func (ds *domainState) computeAllowedConcurrency(cfg DomainLimiterConfig, jobConcurrency int) int {
	base := ds.baseDelay
	if base < cfg.BaseDelay {
		base = cfg.BaseDelay
	}
	effective := ds.effectiveDelay(cfg)
	if effective <= base {
		return jobConcurrency
	}

	diff := effective - base
	reduction := int(diff / cfg.ConcurrencyStep)
	allowed := jobConcurrency - reduction
	if allowed < 1 {
		allowed = 1
	}
	return allowed
}

func (ds *domainState) acquire(ctx context.Context, cfg DomainLimiterConfig, nowFn func() time.Time, req DomainRequest) (time.Duration, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if req.JobConcurrency <= 0 {
		req.JobConcurrency = 1
	}

	for {
		now := nowFn()
		if req.RobotsDelay > 0 {
			robots := req.RobotsDelay
			if robots < cfg.BaseDelay {
				robots = cfg.BaseDelay
			}
			if robots > ds.baseDelay {
				ds.baseDelay = robots
			}
		}
		if ds.adaptiveDelay < ds.baseDelay {
			ds.adaptiveDelay = ds.baseDelay
		}

		waitUntil := ds.nextAvailable
		if ds.backoffUntil.After(waitUntil) {
			waitUntil = ds.backoffUntil
		}
		if waitUntil.After(now) {
			wait := waitUntil.Sub(now)
			ds.mu.Unlock()
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return 0, ctx.Err()
			}
			ds.mu.Lock()
			continue
		}

		js := ds.ensureJobState(req.JobID, req.JobConcurrency)
		js.allowed = ds.computeAllowedConcurrency(cfg, req.JobConcurrency)
		if js.active >= js.allowed {
			ds.cond.Wait()
			continue
		}

		js.active++
		delay := ds.effectiveDelay(cfg)
		ds.nextAvailable = now.Add(delay)
		return delay, nil
	}
}

func (dl *DomainLimiter) release(domain string, jobID string, success bool, rateLimited bool) {
	state := dl.getOrCreateState(domain)

	state.mu.Lock()
	now := dl.now()

	js, ok := state.jobStates[jobID]
	if ok {
		if js.active > 0 {
			js.active--
		}
		state.cond.Broadcast()
	}

	var needPersist bool

	oldAdaptive := state.adaptiveDelay
	oldFloor := state.delayFloor

	if rateLimited {
		state.successStreak = 0
		state.errorStreak++
		if state.probing {
			state.adaptiveDelay = state.probePrevious
			if state.delayFloor < state.probeTarget {
				state.delayFloor = state.probeTarget
			}
			state.probing = false
		}
		nextDelay := state.adaptiveDelay + dl.cfg.DelayStep
		if nextDelay > dl.cfg.MaxAdaptiveDelay {
			nextDelay = dl.cfg.MaxAdaptiveDelay
		}
		if nextDelay > state.adaptiveDelay {
			state.adaptiveDelay = nextDelay
			needPersist = true
		}
		state.backoffUntil = now.Add(state.adaptiveDelay)
	} else if success {
		state.errorStreak = 0
		state.successStreak++
		if state.probing {
			// Probe succeeded, accept lower delay
			state.probing = false
			needPersist = true
		} else if state.successStreak >= dl.cfg.SuccessProbeThreshold {
			proposed := state.adaptiveDelay - dl.cfg.DelayStep
			if proposed < state.delayFloor {
				proposed = state.delayFloor
			}
			if proposed < state.baseDelay {
				proposed = state.baseDelay
			}
			if proposed < state.adaptiveDelay {
				state.probing = true
				state.probePrevious = state.adaptiveDelay
				state.probeTarget = proposed
				state.adaptiveDelay = proposed
				state.successStreak = 0
				needPersist = true
			}
		}
	} else {
		// Non rate-limit failure
		state.successStreak = 0
		state.errorStreak = 0
	}

	adaptiveSeconds := int(state.adaptiveDelay / time.Second)
	floorSeconds := int(state.delayFloor / time.Second)

	adaptiveChanged := state.adaptiveDelay != oldAdaptive
	floorChanged := state.delayFloor != oldFloor
	errorStreak := state.errorStreak
	successStreak := state.successStreak

	shouldPersist := needPersist && (state.lastPersist.IsZero() || now.Sub(state.lastPersist) >= dl.cfg.PersistInterval)
	if shouldPersist {
		state.lastPersist = now
	}
	state.mu.Unlock()

	if adaptiveChanged {
		log.Info().
			Str("domain", domain).
			Int("adaptive_delay_seconds", adaptiveSeconds).
			Int("previous_delay_seconds", int(oldAdaptive/time.Second)).
			Int("error_streak", errorStreak).
			Int("success_streak", successStreak).
			Msg("Updated domain adaptive delay")
	}
	if floorChanged {
		log.Debug().
			Str("domain", domain).
			Int("delay_floor_seconds", floorSeconds).
			Int("previous_floor_seconds", int(oldFloor/time.Second)).
			Msg("Updated domain delay floor")
	}

	if shouldPersist {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := dl.persistDomain(ctx, domain, adaptiveSeconds, floorSeconds); err != nil {
			log.Warn().Err(err).Str("domain", domain).Msg("Failed to persist adaptive delay")
		}
	}
}

func (dl *DomainLimiter) persistDomain(ctx context.Context, domain string, adaptiveDelay int, floor int) error {
	if dl.dbQueue == nil {
		return nil
	}

	return dl.dbQueue.Execute(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
            UPDATE domains
            SET adaptive_delay_seconds = $1,
                adaptive_delay_floor_seconds = $2
            WHERE name = $3
        `, adaptiveDelay, floor, domain)
		return err
	})
}

// Helper utility --------------------------------------------------------------------------------

// IsRateLimitError returns true when error indicates an HTTP 429/403/503 blocking response.
func IsRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "429") ||
		strings.Contains(strings.ToLower(err.Error()), "too many requests") ||
		strings.Contains(strings.ToLower(err.Error()), "rate limit") ||
		strings.Contains(strings.ToLower(err.Error()), "403") ||
		strings.Contains(strings.ToLower(err.Error()), "503")
}
