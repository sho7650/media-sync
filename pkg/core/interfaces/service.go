package interfaces

import (
	"context"
	"io"
	"time"
)

// Service defines the basic contract for all services
type Service interface {
	// Service lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health() ServiceHealth

	// Plugin information
	Info() ServiceInfo
	Capabilities() []Capability
}

// InputService defines contract for services that can retrieve data
type InputService interface {
	Service
	// Data retrieval
	Retrieve(ctx context.Context, req RetrievalRequest) (*DataStream, error)
	// Sync modes
	SupportedModes() []SyncMode
	// Authentication
	Authenticate(ctx context.Context, creds Credentials) error
}

// OutputService defines contract for services that can publish data
type OutputService interface {
	Service
	// Data publishing
	Publish(ctx context.Context, data *DataStream) error
	// Destination configuration
	ConfigureDestination(config DestinationConfig) error
}

// TransformService defines contract for data transformation
type TransformService interface {
	Service
	// Data transformation
	Transform(ctx context.Context, data *DataStream) (*DataStream, error)
	// Schema validation
	ValidateSchema(schema Schema) error
}

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// ServiceInfo provides metadata about a service
type ServiceInfo struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Author      string    `json:"author"`
	CreatedAt   time.Time `json:"created_at"`
}

// Capability describes what a service can do
type Capability struct {
	Type      string                 `json:"type"`
	Supported bool                   `json:"supported"`
	Config    map[string]interface{} `json:"config,omitempty"`
}

// DataStream represents a stream of media data
type DataStream struct {
	ID       string                 `json:"id"`
	Type     MediaType              `json:"type"`
	Metadata map[string]interface{} `json:"metadata"`
	Content  io.ReadCloser          `json:"-"`
	Headers  map[string]string      `json:"headers"`
	Context  StreamContext          `json:"context"`
}

// RetrievalRequest defines parameters for data retrieval
type RetrievalRequest struct {
	ServiceID string                 `json:"service_id"`
	Filters   map[string]interface{} `json:"filters"`
	TimeRange *TimeRange             `json:"time_range,omitempty"`
	BatchSize int                    `json:"batch_size"`
	Cursor    string                 `json:"cursor,omitempty"`
}

// Credentials holds authentication information
type Credentials struct {
	Type     AuthType               `json:"type"`
	Data     map[string]interface{} `json:"data"`
	Metadata CredentialMetadata     `json:"metadata"`
}

// DestinationConfig configures output destinations
type DestinationConfig struct {
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"config"`
}

// Schema defines data validation rules
type Schema struct {
	Type   string                 `json:"type"`
	Fields map[string]interface{} `json:"fields"`
}

// Enums and supporting types
type HealthStatus string

const (
	StatusHealthy HealthStatus = "healthy"
	StatusWarning HealthStatus = "warning"
	StatusError   HealthStatus = "error"
	StatusStopped HealthStatus = "stopped"
)

type MediaType string

const (
	MediaTypePhoto MediaType = "photo"
	MediaTypeVideo MediaType = "video"
	MediaTypeText  MediaType = "text"
	MediaTypeLink  MediaType = "link"
	MediaTypeAudio MediaType = "audio"
)

type SyncMode string

const (
	SyncModeBatch     SyncMode = "batch"
	SyncModeWebhook   SyncMode = "webhook"
	SyncModeStreaming SyncMode = "streaming"
	SyncModeQueue     SyncMode = "queue"
)

type AuthType string

const (
	AuthTypeOAuth2 AuthType = "oauth2"
	AuthTypeJWT    AuthType = "jwt"
	AuthTypeAPIKey AuthType = "apikey"
	AuthTypeBasic  AuthType = "basic"
)

// Supporting structs
type StreamContext struct {
	Source      string    `json:"source"`
	CreatedAt   time.Time `json:"created_at"`
	ProcessedAt time.Time `json:"processed_at"`
}

type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type CredentialMetadata struct {
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Scope     []string  `json:"scope,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
