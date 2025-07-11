package services

import (
	"context"
	"time"
)

// MockCache is a mock implementation of Cache for testing
type MockCache struct {
	PingFunc              func(ctx context.Context) error
	SetFunc               func(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	GetFunc               func(ctx context.Context, key string) (string, error)
	DelFunc               func(ctx context.Context, keys ...string) error
	ExistsFunc            func(ctx context.Context, keys ...string) (bool, error)
	CloseFunc             func() error
	WaitForConnectionFunc func(ctx context.Context) error

	// Track calls for testing
	PingCalls              []context.Context
	SetCalls               []SetCall
	GetCalls               []string
	DelCalls               [][]string
	ExistsCalls            [][]string
	CloseCalls             int
	WaitForConnectionCalls []context.Context
}

type SetCall struct {
	Key        string
	Value      interface{}
	Expiration time.Duration
}

// NewMockCache creates a new mock cache
func NewMockCache() *MockCache {
	return &MockCache{
		PingCalls:              make([]context.Context, 0),
		SetCalls:               make([]SetCall, 0),
		GetCalls:               make([]string, 0),
		DelCalls:               make([][]string, 0),
		ExistsCalls:            make([][]string, 0),
		WaitForConnectionCalls: make([]context.Context, 0),
	}
}

// Ping mocks cache ping
func (m *MockCache) Ping(ctx context.Context) error {
	m.PingCalls = append(m.PingCalls, ctx)

	if m.PingFunc != nil {
		return m.PingFunc(ctx)
	}

	// Default behavior - success
	return nil
}

// Set mocks cache set
func (m *MockCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.SetCalls = append(m.SetCalls, SetCall{
		Key:        key,
		Value:      value,
		Expiration: expiration,
	})

	if m.SetFunc != nil {
		return m.SetFunc(ctx, key, value, expiration)
	}

	// Default behavior - success
	return nil
}

// Get mocks cache get
func (m *MockCache) Get(ctx context.Context, key string) (string, error) {
	m.GetCalls = append(m.GetCalls, key)

	if m.GetFunc != nil {
		return m.GetFunc(ctx, key)
	}

	// Default behavior - return empty string (not found)
	return "", nil
}

// Del mocks cache delete
func (m *MockCache) Del(ctx context.Context, keys ...string) error {
	m.DelCalls = append(m.DelCalls, keys)

	if m.DelFunc != nil {
		return m.DelFunc(ctx, keys...)
	}

	// Default behavior - success
	return nil
}

// Exists mocks cache exists check
func (m *MockCache) Exists(ctx context.Context, keys ...string) (bool, error) {
	m.ExistsCalls = append(m.ExistsCalls, keys)

	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, keys...)
	}

	// Default behavior - keys don't exist
	return false, nil
}

// Close mocks cache close
func (m *MockCache) Close() error {
	m.CloseCalls++

	if m.CloseFunc != nil {
		return m.CloseFunc()
	}

	// Default behavior - success
	return nil
}

// WaitForConnection mocks cache connection waiting
func (m *MockCache) WaitForConnection(ctx context.Context) error {
	m.WaitForConnectionCalls = append(m.WaitForConnectionCalls, ctx)

	if m.WaitForConnectionFunc != nil {
		return m.WaitForConnectionFunc(ctx)
	}

	// Default behavior - success
	return nil
}

// Reset clears all call tracking
func (m *MockCache) Reset() {
	m.PingCalls = make([]context.Context, 0)
	m.SetCalls = make([]SetCall, 0)
	m.GetCalls = make([]string, 0)
	m.DelCalls = make([][]string, 0)
	m.ExistsCalls = make([][]string, 0)
	m.CloseCalls = 0
	m.WaitForConnectionCalls = make([]context.Context, 0)
}

// SetPingError sets up the mock to return an error on Ping
func (m *MockCache) SetPingError(err error) {
	m.PingFunc = func(ctx context.Context) error {
		return err
	}
}

// SetPingSuccess sets up the mock to return success on Ping
func (m *MockCache) SetPingSuccess() {
	m.PingFunc = func(ctx context.Context) error {
		return nil
	}
}

// Ensure MockCache implements Cache interface
var _ Cache = (*MockCache)(nil)
