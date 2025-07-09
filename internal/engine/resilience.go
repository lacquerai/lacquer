package engine

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// RetryConfig defines retry behavior for operations
type RetryConfig struct {
	MaxAttempts     int           `yaml:"max_attempts"`
	InitialDelay    time.Duration `yaml:"initial_delay"`
	MaxDelay        time.Duration `yaml:"max_delay"`
	BackoffFactor   float64       `yaml:"backoff_factor"`
	Jitter          bool          `yaml:"jitter"`
	RetryableErrors []string      `yaml:"retryable_errors"`
}

// CircuitBreakerConfig defines circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold int           `yaml:"failure_threshold"`
	SuccessThreshold int           `yaml:"success_threshold"`
	Timeout          time.Duration `yaml:"timeout"`
	ResetTimeout     time.Duration `yaml:"reset_timeout"`
}

// TimeoutConfig defines timeout behavior for operations
type TimeoutConfig struct {
	DefaultTimeout time.Duration `yaml:"default_timeout"`
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
}

// ResilienceConfig contains all resilience patterns configuration
type ResilienceConfig struct {
	Retry          *RetryConfig          `yaml:"retry"`
	CircuitBreaker *CircuitBreakerConfig `yaml:"circuit_breaker"`
	Timeout        *TimeoutConfig        `yaml:"timeout"`
}

// DefaultResilienceConfig returns default resilience configuration
func DefaultResilienceConfig() *ResilienceConfig {
	return &ResilienceConfig{
		Retry: &RetryConfig{
			MaxAttempts:   3,
			InitialDelay:  100 * time.Millisecond,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
			RetryableErrors: []string{
				"timeout",
				"connection_error",
				"rate_limit",
				"server_error",
				"temporary_failure",
			},
		},
		CircuitBreaker: &CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          60 * time.Second,
			ResetTimeout:     30 * time.Second,
		},
		Timeout: &TimeoutConfig{
			DefaultTimeout: 30 * time.Second,
			ConnectTimeout: 10 * time.Second,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
		},
	}
}

// RetryableError defines an error that can be retried
type RetryableError struct {
	Err        error
	Retryable  bool
	RetryAfter time.Duration
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if retryableErr, ok := err.(*RetryableError); ok {
		return retryableErr.Retryable
	}
	return false
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error, retryable bool) *RetryableError {
	return &RetryableError{
		Err:       err,
		Retryable: retryable,
	}
}

// NewRetryableErrorWithDelay creates a new retryable error with a delay
func NewRetryableErrorWithDelay(err error, retryable bool, retryAfter time.Duration) *RetryableError {
	return &RetryableError{
		Err:        err,
		Retryable:  retryable,
		RetryAfter: retryAfter,
	}
}

// RetryStrategy defines different retry strategies
type RetryStrategy interface {
	NextDelay(attempt int, lastErr error) time.Duration
	ShouldRetry(attempt int, err error) bool
}

// ExponentialBackoffStrategy implements exponential backoff with jitter
type ExponentialBackoffStrategy struct {
	config *RetryConfig
}

// NewExponentialBackoffStrategy creates a new exponential backoff strategy
func NewExponentialBackoffStrategy(config *RetryConfig) *ExponentialBackoffStrategy {
	return &ExponentialBackoffStrategy{config: config}
}

// NextDelay calculates the delay for the next retry attempt
func (s *ExponentialBackoffStrategy) NextDelay(attempt int, lastErr error) time.Duration {
	if retryableErr, ok := lastErr.(*RetryableError); ok && retryableErr.RetryAfter > 0 {
		return retryableErr.RetryAfter
	}

	delay := time.Duration(float64(s.config.InitialDelay) * math.Pow(s.config.BackoffFactor, float64(attempt-1)))

	if delay > s.config.MaxDelay {
		delay = s.config.MaxDelay
	}

	if s.config.Jitter {
		jitter := time.Duration(rand.Float64() * float64(delay) * 0.1)
		delay += jitter
	}

	return delay
}

// ShouldRetry determines if an operation should be retried
func (s *ExponentialBackoffStrategy) ShouldRetry(attempt int, err error) bool {
	if attempt >= s.config.MaxAttempts {
		return false
	}

	if !IsRetryableError(err) {
		return false
	}

	return true
}

// Retrier provides retry functionality for operations
type Retrier struct {
	strategy RetryStrategy
	config   *RetryConfig
}

// NewRetrier creates a new retrier with the given strategy
func NewRetrier(config *RetryConfig) *Retrier {
	if config == nil {
		config = DefaultResilienceConfig().Retry
	}

	return &Retrier{
		strategy: NewExponentialBackoffStrategy(config),
		config:   config,
	}
}

// Execute executes an operation with retry logic
func (r *Retrier) Execute(ctx context.Context, operation func() error) error {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Check context cancellation before each attempt
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := operation()
		if err == nil {
			// Success
			if attempt > 1 {
				log.Info().
					Int("attempt", attempt).
					Int("total_attempts", r.config.MaxAttempts).
					Msg("Operation succeeded after retries")
			}
			return nil
		}

		lastErr = err

		if !r.strategy.ShouldRetry(attempt, err) {
			break
		}

		delay := r.strategy.NextDelay(attempt, err)

		log.Warn().
			Err(err).
			Int("attempt", attempt).
			Int("max_attempts", r.config.MaxAttempts).
			Dur("delay", delay).
			Msg("Operation failed, retrying")

		// Wait for delay with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	log.Error().
		Err(lastErr).
		Int("total_attempts", r.config.MaxAttempts).
		Msg("Operation failed after all retry attempts")

	return fmt.Errorf("operation failed after %d attempts: %w", r.config.MaxAttempts, lastErr)
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config          *CircuitBreakerConfig
	state           CircuitBreakerState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	mu              sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultResilienceConfig().CircuitBreaker
	}

	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// Execute executes an operation through the circuit breaker
func (cb *CircuitBreaker) Execute(ctx context.Context, operation func() error) error {
	// Check if circuit is open
	if !cb.allowRequest() {
		return NewRetryableError(fmt.Errorf("circuit breaker is open"), false)
	}

	err := operation()
	cb.recordResult(err)

	return err
}

// allowRequest determines if a request should be allowed
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if reset timeout has passed
		if time.Since(cb.lastFailureTime) > cb.config.ResetTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.config.ResetTimeout {
				cb.state = StateHalfOpen
				cb.successCount = 0
				log.Info().Msg("Circuit breaker transitioning to half-open")
			}
			cb.mu.Unlock()
			cb.mu.RLock()
			return cb.state == StateHalfOpen
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult records the result of an operation
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err == nil {
		// Success
		cb.onSuccess()
	} else {
		// Failure
		cb.onFailure()
	}
}

// onSuccess handles successful operations
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateClosed:
		cb.failureCount = 0
	case StateHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.state = StateClosed
			cb.failureCount = 0
			cb.successCount = 0
			log.Info().Msg("Circuit breaker closed after successful requests")
		}
	}
}

// onFailure handles failed operations
func (cb *CircuitBreaker) onFailure() {
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		cb.failureCount++
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.state = StateOpen
			log.Warn().
				Int("failure_count", cb.failureCount).
				Int("threshold", cb.config.FailureThreshold).
				Msg("Circuit breaker opened due to failures")
		}
	case StateHalfOpen:
		cb.state = StateOpen
		cb.successCount = 0
		log.Warn().Msg("Circuit breaker opened from half-open due to failure")
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetFailureCount returns the current failure count
func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount
}

// Reset resets the circuit breaker to its initial state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.lastFailureTime = time.Time{}

	log.Info().Msg("Circuit breaker reset")
}

// ResilienceManager combines retry logic, circuit breaker, and timeouts
type ResilienceManager struct {
	retrier        *Retrier
	circuitBreaker *CircuitBreaker
	timeoutConfig  *TimeoutConfig
}

// NewResilienceManager creates a new resilience manager
func NewResilienceManager(config *ResilienceConfig) *ResilienceManager {
	if config == nil {
		config = DefaultResilienceConfig()
	}

	return &ResilienceManager{
		retrier:        NewRetrier(config.Retry),
		circuitBreaker: NewCircuitBreaker(config.CircuitBreaker),
		timeoutConfig:  config.Timeout,
	}
}

// Execute executes an operation with full resilience patterns
func (rm *ResilienceManager) Execute(ctx context.Context, operation func(context.Context) error) error {
	// Apply default timeout if context doesn't have one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rm.timeoutConfig.DefaultTimeout)
		defer cancel()
	}

	// Execute with circuit breaker and retry logic
	return rm.retrier.Execute(ctx, func() error {
		return rm.circuitBreaker.Execute(ctx, func() error {
			return operation(ctx)
		})
	})
}

// ExecuteWithTimeout executes an operation with a specific timeout
func (rm *ResilienceManager) ExecuteWithTimeout(ctx context.Context, timeout time.Duration, operation func(context.Context) error) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return rm.Execute(timeoutCtx, operation)
}

// GetCircuitBreakerState returns the current circuit breaker state
func (rm *ResilienceManager) GetCircuitBreakerState() CircuitBreakerState {
	return rm.circuitBreaker.GetState()
}

// GetCircuitBreakerFailureCount returns the current circuit breaker failure count
func (rm *ResilienceManager) GetCircuitBreakerFailureCount() int {
	return rm.circuitBreaker.GetFailureCount()
}

// ResetCircuitBreaker resets the circuit breaker
func (rm *ResilienceManager) ResetCircuitBreaker() {
	rm.circuitBreaker.Reset()
}
