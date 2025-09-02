package interfaces

import (
	"context"
	"testing"
	"time"
)

// TestServiceInterface tests the basic Service interface contract
func TestServiceInterface(t *testing.T) {
	ctx := context.Background()

	t.Run("Service lifecycle", func(t *testing.T) {
		// This test will initially fail (RED) - no implementation yet
		service := &MockService{}

		// Test Start
		err := service.Start(ctx)
		if err != nil {
			t.Errorf("Start() should not return error for valid service, got: %v", err)
		}

		// Test Health after start
		health := service.Health()
		if health.Status != StatusHealthy {
			t.Errorf("Health() should return healthy after successful start, got: %v", health.Status)
		}

		// Test Stop
		err = service.Stop(ctx)
		if err != nil {
			t.Errorf("Stop() should not return error for running service, got: %v", err)
		}

		// Test Health after stop
		health = service.Health()
		if health.Status != StatusStopped {
			t.Errorf("Health() should return stopped after stop, got: %v", health.Status)
		}
	})

	t.Run("Service info", func(t *testing.T) {
		service := &MockService{}

		info := service.Info()
		if info.Name == "" {
			t.Error("Info().Name should not be empty")
		}
		if info.Version == "" {
			t.Error("Info().Version should not be empty")
		}
		if info.Type == "" {
			t.Error("Info().Type should not be empty")
		}
	})

	t.Run("Service capabilities", func(t *testing.T) {
		service := &MockService{}

		capabilities := service.Capabilities()
		if len(capabilities) == 0 {
			t.Error("Capabilities() should return at least one capability")
		}

		// Validate capability types
		for _, cap := range capabilities {
			if cap.Type == "" {
				t.Error("Capability.Type should not be empty")
			}
		}
	})
}

// TestInputServiceInterface tests InputService-specific functionality
func TestInputServiceInterface(t *testing.T) {
	ctx := context.Background()

	t.Run("Retrieve data", func(t *testing.T) {
		service := NewMockInputService()

		req := RetrievalRequest{
			ServiceID: "test-service",
			Filters:   map[string]interface{}{"type": "photo"},
			BatchSize: 10,
		}

		stream, err := service.Retrieve(ctx, req)
		if err != nil {
			t.Errorf("Retrieve() should not return error for valid request, got: %v", err)
		}
		if stream == nil {
			t.Error("Retrieve() should return non-nil DataStream")
		}
	})

	t.Run("Supported modes", func(t *testing.T) {
		service := NewMockInputService()

		modes := service.SupportedModes()
		if len(modes) == 0 {
			t.Error("SupportedModes() should return at least one sync mode")
		}

		// Check for required batch mode support
		hasBatch := false
		for _, mode := range modes {
			if mode == SyncModeBatch {
				hasBatch = true
				break
			}
		}
		if !hasBatch {
			t.Error("SupportedModes() must include SyncModeBatch")
		}
	})

	t.Run("Authentication", func(t *testing.T) {
		service := NewMockInputService()

		creds := Credentials{
			Type: AuthTypeOAuth2,
			Data: map[string]interface{}{
				"access_token": "test-token",
			},
		}

		err := service.Authenticate(ctx, creds)
		if err != nil {
			t.Errorf("Authenticate() should not return error for valid credentials, got: %v", err)
		}
	})
}

// Mock implementations for testing (will be replaced by real implementations)
type MockService struct {
	isRunning bool
}

func (m *MockService) Start(ctx context.Context) error {
	m.isRunning = true
	return nil
}

func (m *MockService) Stop(ctx context.Context) error {
	m.isRunning = false
	return nil
}

func (m *MockService) Health() ServiceHealth {
	status := StatusStopped
	message := "Mock service is stopped"

	if m.isRunning {
		status = StatusHealthy
		message = "Mock service is healthy"
	}

	return ServiceHealth{
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
	}
}

func (m *MockService) Info() ServiceInfo {
	return ServiceInfo{
		Name:    "mock-service",
		Version: "0.1.0",
		Type:    "mock",
	}
}

func (m *MockService) Capabilities() []Capability {
	return []Capability{
		{Type: "input", Supported: true},
	}
}

type MockInputService struct {
	*MockService
}

func NewMockInputService() *MockInputService {
	return &MockInputService{
		MockService: &MockService{},
	}
}

func (m *MockInputService) Retrieve(ctx context.Context, req RetrievalRequest) (*DataStream, error) {
	return &DataStream{
		ID:   "test-stream",
		Type: MediaTypePhoto,
		Metadata: map[string]interface{}{
			"source": "mock",
		},
	}, nil
}

func (m *MockInputService) SupportedModes() []SyncMode {
	return []SyncMode{SyncModeBatch, SyncModeWebhook}
}

func (m *MockInputService) Authenticate(ctx context.Context, creds Credentials) error {
	return nil
}
