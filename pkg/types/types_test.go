package types

import (
	"testing"
	"time"
)

func TestNotificationDataString(t *testing.T) {
	tests := []struct {
		name string
		data NotificationData
		want string
	}{
		{
			name: "ban action",
			data: NotificationData{IP: "1.2.3.4", Action: "ban", Jail: "ssh"},
			want: "1.2.3.4 banned in ssh",
		},
		{
			name: "unban action",
			data: NotificationData{IP: "5.6.7.8", Action: "unban", Jail: "nginx"},
			want: "5.6.7.8 unbanned in nginx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetLocationString(t *testing.T) {
	tests := []struct {
		name string
		data NotificationData
		want string
	}{
		{name: "empty", data: NotificationData{}, want: ""},
		{name: "country only", data: NotificationData{Country: "Slovakia"}, want: "Slovakia"},
		{name: "city and country", data: NotificationData{City: "Bratislava", Country: "Slovakia"}, want: "Bratislava, Slovakia"},
		{name: "region and country", data: NotificationData{Region: "BA", Country: "Slovakia"}, want: "BA, Slovakia"},
		{name: "all three", data: NotificationData{City: "Bratislava", Region: "BA", Country: "Slovakia"}, want: "Bratislava, BA, Slovakia"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.GetLocationString(); got != tt.want {
				t.Errorf("GetLocationString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name string
		data NotificationData
		want bool
	}{
		{name: "valid", data: NotificationData{IP: "1.2.3.4", Jail: "ssh", Action: "ban"}, want: true},
		{name: "missing IP", data: NotificationData{Jail: "ssh", Action: "ban"}, want: false},
		{name: "missing jail", data: NotificationData{IP: "1.2.3.4", Action: "ban"}, want: false},
		{name: "missing action", data: NotificationData{IP: "1.2.3.4", Jail: "ssh"}, want: false},
		{name: "empty", data: NotificationData{}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBan(t *testing.T) {
	if !(&NotificationData{Action: "ban"}).IsBan() {
		t.Error("IsBan() = false, want true")
	}
	if (&NotificationData{Action: "unban"}).IsBan() {
		t.Error("IsBan() = true, want false")
	}
}

func TestIsUnban(t *testing.T) {
	if !(&NotificationData{Action: "unban"}).IsUnban() {
		t.Error("IsUnban() = false, want true")
	}
	if (&NotificationData{Action: "ban"}).IsUnban() {
		t.Error("IsUnban() = true, want false")
	}
}

func TestToJSON(t *testing.T) {
	data := &NotificationData{
		IP:      "1.2.3.4",
		Jail:    "ssh",
		Action:  "ban",
		Country: "Slovakia",
	}

	jsonBytes, err := data.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() returned error: %v", err)
	}

	jsonStr := string(jsonBytes)
	if jsonStr == "" {
		t.Fatal("ToJSON() returned empty bytes")
	}

	if len(jsonBytes) == 0 {
		t.Error("ToJSON() returned zero-length bytes")
	}
}

func TestBatchResultIsSuccess(t *testing.T) {
	tests := []struct {
		name string
		br   BatchResult
		want bool
	}{
		{name: "no failures", br: BatchResult{FailedCount: 0}, want: true},
		{name: "has failures", br: BatchResult{FailedCount: 2}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.br.IsSuccess(); got != tt.want {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBatchResultGetSuccessRate(t *testing.T) {
	tests := []struct {
		name string
		br   BatchResult
		want float64
	}{
		{name: "zero total", br: BatchResult{}, want: 0},
		{name: "all succeeded", br: BatchResult{TotalConnectors: 4, SuccessfulCount: 4}, want: 100},
		{name: "half succeeded", br: BatchResult{TotalConnectors: 4, SuccessfulCount: 2}, want: 50},
		{name: "none succeeded", br: BatchResult{TotalConnectors: 3, SuccessfulCount: 0}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.br.GetSuccessRate(); got != tt.want {
				t.Errorf("GetSuccessRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetFailedConnectors(t *testing.T) {
	br := BatchResult{
		Results: []ExecutionResult{
			{ConnectorName: "discord", Success: true},
			{ConnectorName: "slack", Success: false},
			{ConnectorName: "teams", Success: true},
			{ConnectorName: "email", Success: false},
		},
	}

	failed := br.GetFailedConnectors()
	expected := []string{"slack", "email"}

	if len(failed) != len(expected) {
		t.Fatalf("GetFailedConnectors() returned %v, want %v", failed, expected)
	}
	for i := range expected {
		if failed[i] != expected[i] {
			t.Errorf("GetFailedConnectors()[%d] = %q, want %q", i, failed[i], expected[i])
		}
	}
}

func TestHealthStatusIsHealthy(t *testing.T) {
	if !(&HealthStatus{Status: "healthy"}).IsHealthy() {
		t.Error("IsHealthy() = false for status 'healthy'")
	}
	if (&HealthStatus{Status: "degraded"}).IsHealthy() {
		t.Error("IsHealthy() = true for status 'degraded'")
	}
}

func TestConnectorMetricsGetSuccessRate(t *testing.T) {
	tests := []struct {
		name string
		cm   ConnectorMetrics
		want float64
	}{
		{name: "zero executions", cm: ConnectorMetrics{}, want: 0},
		{name: "all succeeded", cm: ConnectorMetrics{Executions: 10, Successes: 10}, want: 100},
		{name: "half succeeded", cm: ConnectorMetrics{Executions: 4, Successes: 2}, want: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cm.GetSuccessRate(); got != tt.want {
				t.Errorf("GetSuccessRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTemplateVarsDefaultValues(t *testing.T) {
	v := TemplateVars{
		IP:     "1.2.3.4",
		Jail:   "ssh",
		Action: "ban",
		Time:   time.Now(),
	}

	if v.IP != "1.2.3.4" {
		t.Errorf("TemplateVars.IP = %q, want %q", v.IP, "1.2.3.4")
	}
	if v.Jail != "ssh" {
		t.Errorf("TemplateVars.Jail = %q, want %q", v.Jail, "ssh")
	}
}

func TestAPIResponseSuccess(t *testing.T) {
	resp := APIResponse{
		Success: true,
		Message: "ok",
		Version: "1.0.0",
	}

	if !resp.Success {
		t.Error("APIResponse.Success = false, want true")
	}
	if resp.Message != "ok" {
		t.Errorf("APIResponse.Message = %q, want %q", resp.Message, "ok")
	}
}
