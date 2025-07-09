package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultResilienceConfig(t *testing.T) {
	config := DefaultResilienceConfig()

	assert.NotNil(t, config.Retry)
	assert.Equal(t, 3, config.Retry.MaxAttempts)
	assert.Equal(t, 100*time.Millisecond, config.Retry.InitialDelay)
	assert.Equal(t, 30*time.Second, config.Retry.MaxDelay)
	assert.Equal(t, 2.0, config.Retry.BackoffFactor)
	assert.True(t, config.Retry.Jitter)

	assert.NotNil(t, config.CircuitBreaker)
	assert.Equal(t, 5, config.CircuitBreaker.FailureThreshold)
	assert.Equal(t, 3, config.CircuitBreaker.SuccessThreshold)
	assert.Equal(t, 60*time.Second, config.CircuitBreaker.Timeout)
	assert.Equal(t, 30*time.Second, config.CircuitBreaker.ResetTimeout)

	assert.NotNil(t, config.Timeout)
	assert.Equal(t, 30*time.Second, config.Timeout.DefaultTimeout)
	assert.Equal(t, 10*time.Second, config.Timeout.ConnectTimeout)
}

func TestRetryableError(t *testing.T) {
	originalErr := errors.New("test error")

	// Test retryable error
	retryableErr := NewRetryableError(originalErr, true)
	assert.True(t, IsRetryableError(retryableErr))
	assert.Equal(t, originalErr.Error(), retryableErr.Error())
	assert.Equal(t, originalErr, retryableErr.Unwrap())

	// Test non-retryable error
	nonRetryableErr := NewRetryableError(originalErr, false)
	assert.False(t, IsRetryableError(nonRetryableErr))

	// Test regular error
	regularErr := errors.New("regular error")
	assert.False(t, IsRetryableError(regularErr))

	// Test retryable error with delay
	retryAfter := 5 * time.Second
	delayedErr := NewRetryableErrorWithDelay(originalErr, true, retryAfter)
	assert.True(t, IsRetryableError(delayedErr))
	assert.Equal(t, retryAfter, delayedErr.RetryAfter)
}

func TestExponentialBackoffStrategy(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        false, // Disable jitter for predictable testing
	}

	strategy := NewExponentialBackoffStrategy(config)

	// Test delay calculation
	delay1 := strategy.NextDelay(1, errors.New("test"))
	assert.Equal(t, 100*time.Millisecond, delay1)

	delay2 := strategy.NextDelay(2, errors.New("test"))
	assert.Equal(t, 200*time.Millisecond, delay2)

	delay3 := strategy.NextDelay(3, errors.New("test"))
	assert.Equal(t, 400*time.Millisecond, delay3)

	// Test max delay cap
	delay10 := strategy.NextDelay(10, errors.New("test"))
	assert.Equal(t, config.MaxDelay, delay10)

	// Test retry after header
	retryAfterErr := NewRetryableErrorWithDelay(errors.New("rate limit"), true, 5*time.Second)
	delayAfter := strategy.NextDelay(1, retryAfterErr)
	assert.Equal(t, 5*time.Second, delayAfter)

	// Test should retry
	retryableErr := NewRetryableError(errors.New("retryable"), true)
	assert.True(t, strategy.ShouldRetry(1, retryableErr))
	assert.True(t, strategy.ShouldRetry(4, retryableErr))
	assert.False(t, strategy.ShouldRetry(5, retryableErr)) // Max attempts reached

	nonRetryableErr := NewRetryableError(errors.New("non-retryable"), false)
	assert.False(t, strategy.ShouldRetry(1, nonRetryableErr))
}

func TestRetrier_Success(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
	}

	retrier := NewRetrier(config)
	ctx := context.Background()

	callCount := 0
	operation := func() error {
		callCount++
		return nil // Success on first try
	}

	err := retrier.Execute(ctx, operation)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestRetrier_RetryAndSuccess(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
	}

	retrier := NewRetrier(config)
	ctx := context.Background()

	callCount := 0
	operation := func() error {
		callCount++
		if callCount < 3 {
			return NewRetryableError(errors.New("temporary failure"), true)
		}
		return nil // Success on third try
	}

	err := retrier.Execute(ctx, operation)
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestRetrier_MaxAttemptsExceeded(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
	}

	retrier := NewRetrier(config)
	ctx := context.Background()

	callCount := 0
	operation := func() error {
		callCount++
		return NewRetryableError(errors.New("persistent failure"), true)
	}

	err := retrier.Execute(ctx, operation)
	assert.Error(t, err)
	assert.Equal(t, 3, callCount)
	assert.Contains(t, err.Error(), "operation failed after 3 attempts")
}

func TestRetrier_NonRetryableError(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
	}

	retrier := NewRetrier(config)
	ctx := context.Background()

	callCount := 0
	operation := func() error {
		callCount++
		return NewRetryableError(errors.New("non-retryable failure"), false)
	}

	err := retrier.Execute(ctx, operation)
	assert.Error(t, err)
	assert.Equal(t, 1, callCount) // Should not retry
}

func TestRetrier_ContextCancellation(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  100 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
	}

	retrier := NewRetrier(config)
	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	operation := func() error {
		callCount++
		if callCount == 2 {
			cancel() // Cancel after second attempt
		}
		return NewRetryableError(errors.New("failure"), true)
	}

	err := retrier.Execute(ctx, operation)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Equal(t, 2, callCount)
}

func TestCircuitBreaker_InitialState(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          60 * time.Second,
		ResetTimeout:     30 * time.Second,
	}

	cb := NewCircuitBreaker(config)
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, 0, cb.GetFailureCount())
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          60 * time.Second,
		ResetTimeout:     10 * time.Millisecond, // Short for testing
	}

	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Initially closed
	assert.Equal(t, StateClosed, cb.GetState())

	// Add failures to trigger opening
	for i := 0; i < 3; i++ {
		err := cb.Execute(ctx, func() error {
			return errors.New("failure")
		})
		assert.Error(t, err)
	}

	// Should be open now
	assert.Equal(t, StateOpen, cb.GetState())
	assert.Equal(t, 3, cb.GetFailureCount())

	// Requests should be rejected
	err := cb.Execute(ctx, func() error {
		return nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")

	// Wait for reset timeout
	time.Sleep(15 * time.Millisecond)

	// Should transition to half-open on next request
	err = cb.Execute(ctx, func() error {
		return nil // Success
	})
	assert.NoError(t, err)
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Add successful requests to close circuit
	err = cb.Execute(ctx, func() error {
		return nil // Success
	})
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          60 * time.Second,
		ResetTimeout:     10 * time.Millisecond,
	}

	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Trigger opening
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func() error {
			return errors.New("failure")
		})
	}
	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for reset timeout
	time.Sleep(15 * time.Millisecond)

	// First request should transition to half-open
	err := cb.Execute(ctx, func() error {
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Failure in half-open should immediately open circuit
	err = cb.Execute(ctx, func() error {
		return errors.New("failure")
	})
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.GetState())
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := &CircuitBreakerConfig{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          60 * time.Second,
		ResetTimeout:     30 * time.Second,
	}

	cb := NewCircuitBreaker(config)
	ctx := context.Background()

	// Trigger opening
	for i := 0; i < 2; i++ {
		cb.Execute(ctx, func() error {
			return errors.New("failure")
		})
	}
	assert.Equal(t, StateOpen, cb.GetState())
	assert.Equal(t, 2, cb.GetFailureCount())

	// Reset circuit breaker
	cb.Reset()
	assert.Equal(t, StateClosed, cb.GetState())
	assert.Equal(t, 0, cb.GetFailureCount())
}

func TestResilienceManager_Success(t *testing.T) {
	config := &ResilienceConfig{
		Retry: &RetryConfig{
			MaxAttempts:   3,
			InitialDelay:  10 * time.Millisecond,
			MaxDelay:      1 * time.Second,
			BackoffFactor: 2.0,
		},
		CircuitBreaker: &CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          60 * time.Second,
			ResetTimeout:     30 * time.Second,
		},
		Timeout: &TimeoutConfig{
			DefaultTimeout: 5 * time.Second,
		},
	}

	rm := NewResilienceManager(config)
	ctx := context.Background()

	callCount := 0
	err := rm.Execute(ctx, func(context.Context) error {
		callCount++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, StateClosed, rm.GetCircuitBreakerState())
}

func TestResilienceManager_RetryAndSuccess(t *testing.T) {
	config := &ResilienceConfig{
		Retry: &RetryConfig{
			MaxAttempts:   3,
			InitialDelay:  10 * time.Millisecond,
			MaxDelay:      1 * time.Second,
			BackoffFactor: 2.0,
		},
		CircuitBreaker: &CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          60 * time.Second,
			ResetTimeout:     30 * time.Second,
		},
		Timeout: &TimeoutConfig{
			DefaultTimeout: 5 * time.Second,
		},
	}

	rm := NewResilienceManager(config)
	ctx := context.Background()

	callCount := 0
	err := rm.Execute(ctx, func(context.Context) error {
		callCount++
		if callCount < 3 {
			return NewRetryableError(errors.New("temporary failure"), true)
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestResilienceManager_CircuitBreakerTriggered(t *testing.T) {
	config := &ResilienceConfig{
		Retry: &RetryConfig{
			MaxAttempts:   2,
			InitialDelay:  10 * time.Millisecond,
			MaxDelay:      1 * time.Second,
			BackoffFactor: 2.0,
		},
		CircuitBreaker: &CircuitBreakerConfig{
			FailureThreshold: 2, // Low threshold for testing
			SuccessThreshold: 3,
			Timeout:          60 * time.Second,
			ResetTimeout:     30 * time.Second,
		},
		Timeout: &TimeoutConfig{
			DefaultTimeout: 5 * time.Second,
		},
	}

	rm := NewResilienceManager(config)
	ctx := context.Background()

	// Execute enough failures to trigger circuit breaker
	callCount := 0
	for i := 0; i < 3; i++ {
		rm.Execute(ctx, func(context.Context) error {
			callCount++
			return NewRetryableError(errors.New("persistent failure"), true)
		})
	}

	// Circuit should be open now
	assert.Equal(t, StateOpen, rm.GetCircuitBreakerState())
	assert.Equal(t, 2, rm.GetCircuitBreakerFailureCount())

	// Further requests should be rejected immediately
	initialCallCount := callCount
	err := rm.Execute(ctx, func(context.Context) error {
		callCount++
		return nil
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
	assert.Equal(t, initialCallCount, callCount) // No additional calls
}

func TestResilienceManager_Timeout(t *testing.T) {
	config := &ResilienceConfig{
		Retry: &RetryConfig{
			MaxAttempts:   1, // No retries for this test
			InitialDelay:  10 * time.Millisecond,
			MaxDelay:      1 * time.Second,
			BackoffFactor: 2.0,
		},
		CircuitBreaker: &CircuitBreakerConfig{
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          60 * time.Second,
			ResetTimeout:     30 * time.Second,
		},
		Timeout: &TimeoutConfig{
			DefaultTimeout: 50 * time.Millisecond,
		},
	}

	rm := NewResilienceManager(config)
	ctx := context.Background()

	// Test that timeout context is properly created by checking the deadline
	var hasTimeoutDeadline bool

	err := rm.Execute(ctx, func(opCtx context.Context) error {
		// Check if the context passed to the operation has a timeout
		_, hasDeadline := opCtx.Deadline()
		hasTimeoutDeadline = hasDeadline
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, hasTimeoutDeadline, "Expected operation to receive context with timeout deadline")
}

func TestResilienceManager_ExecuteWithTimeout(t *testing.T) {
	rm := NewResilienceManager(nil) // Use defaults
	ctx := context.Background()

	err := rm.ExecuteWithTimeout(ctx, 50*time.Millisecond, func(opCtx context.Context) error {
		select {
		case <-time.After(100 * time.Millisecond):
			return nil
		case <-opCtx.Done():
			return opCtx.Err()
		}
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestResilienceManager_ResetCircuitBreaker(t *testing.T) {
	config := &ResilienceConfig{
		Retry: &RetryConfig{
			MaxAttempts:   1,
			InitialDelay:  10 * time.Millisecond,
			MaxDelay:      1 * time.Second,
			BackoffFactor: 2.0,
		},
		CircuitBreaker: &CircuitBreakerConfig{
			FailureThreshold: 2,
			SuccessThreshold: 3,
			Timeout:          60 * time.Second,
			ResetTimeout:     30 * time.Second,
		},
		Timeout: &TimeoutConfig{
			DefaultTimeout: 5 * time.Second,
		},
	}

	rm := NewResilienceManager(config)
	ctx := context.Background()

	// Trigger circuit breaker opening
	for i := 0; i < 2; i++ {
		rm.Execute(ctx, func(context.Context) error {
			return NewRetryableError(errors.New("failure"), true)
		})
	}

	assert.Equal(t, StateOpen, rm.GetCircuitBreakerState())

	// Reset circuit breaker
	rm.ResetCircuitBreaker()
	assert.Equal(t, StateClosed, rm.GetCircuitBreakerState())
	assert.Equal(t, 0, rm.GetCircuitBreakerFailureCount())
}

// Benchmark tests
func BenchmarkRetrier_Success(b *testing.B) {
	retrier := NewRetrier(nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		retrier.Execute(ctx, func() error {
			return nil
		})
	}
}

func BenchmarkCircuitBreaker_Closed(b *testing.B) {
	cb := NewCircuitBreaker(nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(ctx, func() error {
			return nil
		})
	}
}

func BenchmarkResilienceManager_Success(b *testing.B) {
	rm := NewResilienceManager(nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.Execute(ctx, func(context.Context) error {
			return nil
		})
	}
}
