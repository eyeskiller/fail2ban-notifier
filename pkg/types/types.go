package types

import (
	"encoding/json"
	"time"
)

type NotificationData struct {
	IP        string    `json:"ip"`
	Jail      string    `json:"jail"`
	Action    string    `json:"action"` // "ban" or "unban"
	Time      time.Time `json:"time"`
	Country   string    `json:"country"`
	Region    string    `json:"region"`
	City      string    `json:"city"`
	ISP       string    `json:"isp"`
	Hostname  string    `json:"hostname,omitempty"`
	Failures  int       `json:"failures,omitempty"`
	Timezone  string    `json:"timezone,nil"`
	Latitude  float64   `json:"latitude,nil"`
	Longitude float64   `json:"longitude,nil"`
}

// String returns a string representation of the notification data
func (nd *NotificationData) String() string {
	return nd.IP + " " + nd.Action + "ned in " + nd.Jail
}

// GetLocationString returns a formatted location string
func (nd *NotificationData) GetLocationString() string {
	if nd.Country == "" {
		return ""
	}

	if nd.City != "" && nd.Region != "" {
		return nd.City + ", " + nd.Region + ", " + nd.Country
	} else if nd.City != "" {
		return nd.City + ", " + nd.Country
	} else if nd.Region != "" {
		return nd.Region + ", " + nd.Country
	}

	return nd.Country
}

// IsValid checks if the notification data has required fields
func (nd *NotificationData) IsValid() bool {
	return nd.IP != "" && nd.Jail != "" && nd.Action != ""
}

// IsBan returns true if this is a ban action
func (nd *NotificationData) IsBan() bool {
	return nd.Action == "ban"
}

// IsUnban returns true if this is an unban action
func (nd *NotificationData) IsUnban() bool {
	return nd.Action == "unban"
}

// ToJSON returns the notification data as JSON
func (nd *NotificationData) ToJSON() ([]byte, error) {
	return json.Marshal(nd)
}

// ExecutionResult represents the result of a connector execution
type ExecutionResult struct {
	ConnectorName string        `json:"connector_name"`
	Success       bool          `json:"success"`
	Error         string        `json:"error,omitempty"`
	Duration      time.Duration `json:"duration"`
	Timestamp     time.Time     `json:"timestamp"`
	Attempts      int           `json:"attempts"`
}

// BatchResult represents the result of executing multiple connectors
type BatchResult struct {
	TotalConnectors  int               `json:"total_connectors"`
	SuccessfulCount  int               `json:"successful_count"`
	FailedCount      int               `json:"failed_count"`
	TotalDuration    time.Duration     `json:"total_duration"`
	Results          []ExecutionResult `json:"results"`
	NotificationData NotificationData  `json:"notification_data"`
	Timestamp        time.Time         `json:"timestamp"`
}

// IsSuccess returns true if all connectors executed successfully
func (br *BatchResult) IsSuccess() bool {
	return br.FailedCount == 0
}

// GetSuccessRate returns the success rate as a percentage
func (br *BatchResult) GetSuccessRate() float64 {
	if br.TotalConnectors == 0 {
		return 0
	}
	return float64(br.SuccessfulCount) / float64(br.TotalConnectors) * 100
}

// GetFailedConnectors returns a list of failed connector names
func (br *BatchResult) GetFailedConnectors() []string {
	var failed []string
	for _, result := range br.Results {
		if !result.Success {
			failed = append(failed, result.ConnectorName)
		}
	}
	return failed
}

// LogEntry represents a log entry for monitoring
type LogEntry struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// HealthStatus represents the health status of the system
type HealthStatus struct {
	Status        string            `json:"status"` // "healthy", "degraded", "unhealthy"
	Version       string            `json:"version"`
	Uptime        time.Duration     `json:"uptime"`
	Connectors    int               `json:"connectors"`
	LastExecution *time.Time        `json:"last_execution,omitempty"`
	Errors        []string          `json:"errors,omitempty"`
	Checks        map[string]string `json:"checks"`
}

// IsHealthy returns true if the system is healthy
func (hs *HealthStatus) IsHealthy() bool {
	return hs.Status == "healthy"
}

// Metrics represents system metrics
type Metrics struct {
	TotalNotifications      int64                       `json:"total_notifications"`
	SuccessfulNotifications int64                       `json:"successful_notifications"`
	FailedNotifications     int64                       `json:"failed_notifications"`
	ConnectorMetrics        map[string]ConnectorMetrics `json:"connector_metrics"`
	GeoIPCacheHits          int64                       `json:"geoip_cache_hits"`
	GeoIPCacheMisses        int64                       `json:"geoip_cache_misses"`
	AverageExecutionTime    time.Duration               `json:"average_execution_time"`
	LastReset               time.Time                   `json:"last_reset"`
}

// ConnectorMetrics represents metrics for a specific connector
type ConnectorMetrics struct {
	Executions          int64         `json:"executions"`
	Successes           int64         `json:"successes"`
	Failures            int64         `json:"failures"`
	AverageTime         time.Duration `json:"average_time"`
	LastExecution       *time.Time    `json:"last_execution,omitempty"`
	LastError           string        `json:"last_error,omitempty"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
}

// GetSuccessRate returns the success rate for a connector
func (cm *ConnectorMetrics) GetSuccessRate() float64 {
	if cm.Executions == 0 {
		return 0
	}
	return float64(cm.Successes) / float64(cm.Executions) * 100
}

type ConfigSummary struct {
	Version           string   `json:"version"`
	ConnectorPath     string   `json:"connector_path"`
	EnabledConnectors []string `json:"enabled_connectors"`
	GeoIPEnabled      bool     `json:"geoip_enabled"`
	GeoIPService      string   `json:"geoip_service"`
	Debug             bool     `json:"debug"`
	TotalConnectors   int      `json:"total_connectors"`
}

// Event types for monitoring and webhooks
type Event struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
	Severity  string                 `json:"severity"` // "info", "warning", "error", "critical"
}

type TemplateVars struct {
	IP          string    `json:"ip"`
	Jail        string    `json:"jail"`
	Action      string    `json:"action"`
	Time        time.Time `json:"time"`
	Country     string    `json:"country"`
	Region      string    `json:"region"`
	City        string    `json:"city"`
	ISP         string    `json:"isp"`
	Hostname    string    `json:"hostname"`
	Failures    int       `json:"failures"`
	Location    string    `json:"location"`
	Timestamp   int64     `json:"timestamp"`
	TimeString  string    `json:"time_string"`
	ActionEmoji string    `json:"action_emoji"`
	ActionColor string    `json:"action_color"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Version   string      `json:"version,omitempty"`
}

