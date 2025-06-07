package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NotificationData contains information about the fail2ban event
type NotificationData struct {
	IP         string    `json:"ip"`
	Jail       string    `json:"jail"`
	Action     string    `json:"action"` // "ban" or "unban"
	Time       time.Time `json:"time"`
	Country    string    `json:"country,omitempty"`
	Region     string    `json:"region,omitempty"`
	City       string    `json:"city,omitempty"`
	ISP        string    `json:"isp,omitempty"`
	Hostname   string    `json:"hostname,omitempty"`
	Failures   int       `json:"failures,omitempty"`
}

// Config represents the application configuration
type Config struct {
	Connectors map[string]ConnectorConfig `json:"connectors"`
	GeoIP      GeoIPConfig                `json:"geoip"`
	Debug      bool                       `json:"debug"`
}

// ConnectorConfig defines a notification connector
type ConnectorConfig struct {
	Type     string            `json:"type"`
	Enabled  bool              `json:"enabled"`
	Settings map[string]string `json:"settings"`
}

// GeoIPConfig contains geolocation API settings
type GeoIPConfig struct {
	Enabled bool   `json:"enabled"`
	APIKey  string `json:"api_key,omitempty"`
	Service string `json:"service"` // "ipapi" or "ipgeolocation"
}

// Notifier interface for different notification services
type Notifier interface {
	Send(data NotificationData) error
	GetName() string
}

// DiscordNotifier sends notifications to Discord
type DiscordNotifier struct {
	WebhookURL string
	Username   string
	AvatarURL  string
}

func (d *DiscordNotifier) GetName() string {
	return "Discord"
}

func (d *DiscordNotifier) Send(data NotificationData) error {
	color := 0xff4444 // Red for ban
	if data.Action == "unban" {
		color = 0x44ff44 // Green for unban
	}

	location := ""
	if data.Country != "" {
		location = fmt.Sprintf(" from %s", data.Country)
		if data.City != "" {
			location = fmt.Sprintf(" from %s, %s", data.City, data.Country)
		}
	}

	embed := map[string]interface{}{
		"title":       fmt.Sprintf("Fail2Ban %s: %s", strings.Title(data.Action), data.Jail),
		"description": fmt.Sprintf("IP **%s**%s has been %sed", data.IP, location, data.Action),
		"color":       color,
		"timestamp":   data.Time.Format(time.RFC3339),
		"fields": []map[string]interface{}{
			{"name": "IP Address", "value": data.IP, "inline": true},
			{"name": "Jail", "value": data.Jail, "inline": true},
			{"name": "Action", "value": strings.Title(data.Action), "inline": true},
		},
	}

	if data.Failures > 0 {
		embed["fields"] = append(embed["fields"].([]map[string]interface{}),
			map[string]interface{}{"name": "Failures", "value": fmt.Sprintf("%d", data.Failures), "inline": true})
	}

	if data.ISP != "" {
		embed["fields"] = append(embed["fields"].([]map[string]interface{}),
			map[string]interface{}{"name": "ISP", "value": data.ISP, "inline": true})
	}

	payload := map[string]interface{}{
		"username":   d.Username,
		"avatar_url": d.AvatarURL,
		"embeds":     []interface{}{embed},
	}

	return d.sendWebhook(payload)
}

func (d *DiscordNotifier) sendWebhook(payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(d.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("discord webhook failed: %s - %s", resp.Status, string(body))
	}

	return nil
}

// TeamsNotifier sends notifications to Microsoft Teams
type TeamsNotifier struct {
	WebhookURL string
}

func (t *TeamsNotifier) GetName() string {
	return "Teams"
}

func (t *TeamsNotifier) Send(data NotificationData) error {
	themeColor := "FF4444" // Red for ban
	if data.Action == "unban" {
		themeColor = "44FF44" // Green for unban
	}

	location := ""
	if data.Country != "" {
		location = fmt.Sprintf(" from %s", data.Country)
		if data.City != "" {
			location = fmt.Sprintf(" from %s, %s", data.City, data.Country)
		}
	}

	facts := []map[string]interface{}{
		{"name": "IP Address", "value": data.IP},
		{"name": "Jail", "value": data.Jail},
		{"name": "Action", "value": strings.Title(data.Action)},
		{"name": "Time", "value": data.Time.Format("2006-01-02 15:04:05 MST")},
	}

	if data.Failures > 0 {
		facts = append(facts, map[string]interface{}{"name": "Failures", "value": fmt.Sprintf("%d", data.Failures)})
	}
	if data.ISP != "" {
		facts = append(facts, map[string]interface{}{"name": "ISP", "value": data.ISP})
	}

	payload := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"themeColor": themeColor,
		"summary":    fmt.Sprintf("Fail2Ban %s: %s", strings.Title(data.Action), data.IP),
		"sections": []map[string]interface{}{
			{
				"activityTitle":    fmt.Sprintf("Fail2Ban %s Alert", strings.Title(data.Action)),
				"activitySubtitle": fmt.Sprintf("IP %s%s has been %sed in jail '%s'", data.IP, location, data.Action, data.Jail),
				"facts":            facts,
				"markdown":         true,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(t.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("teams webhook failed: %s - %s", resp.Status, string(body))
	}

	return nil
}

// SlackNotifier sends notifications to Slack
type SlackNotifier struct {
	WebhookURL string
	Channel    string
	Username   string
	IconEmoji  string
}

func (s *SlackNotifier) GetName() string {
	return "Slack"
}

func (s *SlackNotifier) Send(data NotificationData) error {
	color := "danger" // Red for ban
	if data.Action == "unban" {
		color = "good" // Green for unban
	}

	location := ""
	if data.Country != "" {
		location = fmt.Sprintf(" from %s", data.Country)
		if data.City != "" {
			location = fmt.Sprintf(" from %s, %s", data.City, data.Country)
		}
	}

	fields := []map[string]interface{}{
		{"title": "IP Address", "value": data.IP, "short": true},
		{"title": "Jail", "value": data.Jail, "short": true},
		{"title": "Action", "value": strings.Title(data.Action), "short": true},
		{"title": "Time", "value": data.Time.Format("2006-01-02 15:04:05 MST"), "short": true},
	}

	if data.Failures > 0 {
		fields = append(fields, map[string]interface{}{"title": "Failures", "value": fmt.Sprintf("%d", data.Failures), "short": true})
	}
	if data.ISP != "" {
		fields = append(fields, map[string]interface{}{"title": "ISP", "value": data.ISP, "short": true})
	}

	attachment := map[string]interface{}{
		"color":      color,
		"title":      fmt.Sprintf("Fail2Ban %s Alert", strings.Title(data.Action)),
		"text":       fmt.Sprintf("IP *%s*%s has been %sed in jail '%s'", data.IP, location, data.Action, data.Jail),
		"fields":     fields,
		"ts":         data.Time.Unix(),
		"footer":     "Fail2Ban Notifier",
		"mrkdwn_in":  []string{"text"},
	}

	payload := map[string]interface{}{
		"channel":     s.Channel,
		"username":    s.Username,
		"icon_emoji":  s.IconEmoji,
		"attachments": []interface{}{attachment},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(s.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack webhook failed: %s - %s", resp.Status, string(body))
	}

	return nil
}

// TelegramNotifier sends notifications to Telegram
type TelegramNotifier struct {
	BotToken string
	ChatID   string
}

func (tg *TelegramNotifier) GetName() string {
	return "Telegram"
}

func (tg *TelegramNotifier) Send(data NotificationData) error {
	emoji := "🚫" // Ban emoji
	if data.Action == "unban" {
		emoji = "✅" // Unban emoji
	}

	location := ""
	if data.Country != "" {
		location = fmt.Sprintf(" from %s", data.Country)
		if data.City != "" {
			location = fmt.Sprintf(" from %s, %s", data.City, data.Country)
		}
	}

	text := fmt.Sprintf("%s *Fail2Ban %s Alert*\n\n", emoji, strings.Title(data.Action))
	text += fmt.Sprintf("🌐 *IP:* `%s`%s\n", data.IP, location)
	text += fmt.Sprintf("🔒 *Jail:* %s\n", data.Jail)
	text += fmt.Sprintf("⚡ *Action:* %s\n", strings.Title(data.Action))
	text += fmt.Sprintf("🕐 *Time:* %s\n", data.Time.Format("2006-01-02 15:04:05 MST"))

	if data.Failures > 0 {
		text += fmt.Sprintf("❌ *Failures:* %d\n", data.Failures)
	}
	if data.ISP != "" {
		text += fmt.Sprintf("🏢 *ISP:* %s\n", data.ISP)
	}

	payload := map[string]interface{}{
		"chat_id":    tg.ChatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tg.BotToken)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API failed: %s - %s", resp.Status, string(body))
	}

	return nil
}

// CustomNotifier allows users to define their own notification logic
type CustomNotifier struct {
	Name       string
	WebhookURL string
	Headers    map[string]string
	Template   string
}

func (c *CustomNotifier) GetName() string {
	return c.Name
}

func (c *CustomNotifier) Send(data NotificationData) error {
	// Simple template replacement
	body := c.Template
	body = strings.ReplaceAll(body, "{{.IP}}", data.IP)
	body = strings.ReplaceAll(body, "{{.Jail}}", data.Jail)
	body = strings.ReplaceAll(body, "{{.Action}}", data.Action)
	body = strings.ReplaceAll(body, "{{.Time}}", data.Time.Format(time.RFC3339))
	body = strings.ReplaceAll(body, "{{.Country}}", data.Country)
	body = strings.ReplaceAll(body, "{{.City}}", data.City)
	body = strings.ReplaceAll(body, "{{.ISP}}", data.ISP)
	body = strings.ReplaceAll(body, "{{.Failures}}", fmt.Sprintf("%d", data.Failures))

	req, err := http.NewRequest("POST", c.WebhookURL, strings.NewReader(body))
	if err != nil {
		return err
	}

	// Set headers
	for key, value := range c.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("custom webhook failed: %s - %s", resp.Status, string(respBody))
	}

	return nil
}

// GeoIP service to get location information
func getGeoIPInfo(ip string, config GeoIPConfig) (country, region, city, isp string) {
	if !config.Enabled {
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var url string

	switch config.Service {
	case "ipgeolocation":
		url = fmt.Sprintf("https://api.ipgeolocation.io/ipgeo?apiKey=%s&ip=%s", config.APIKey, ip)
	default: // ipapi
		url = fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,regionName,city,isp", ip)
	}

	resp, err := client.Get(url)
	if err != nil {
		log.Printf("Failed to get geo info: %v", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read geo response: %v", err)
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("Failed to parse geo response: %v", err)
		return
	}

	if config.Service == "ipgeolocation" {
		if v, ok := result["country_name"].(string); ok {
			country = v
		}
		if v, ok := result["state_prov"].(string); ok {
			region = v
		}
		if v, ok := result["city"].(string); ok {
			city = v
		}
		if v, ok := result["isp"].(string); ok {
			isp = v
		}
	} else {
		if status, ok := result["status"].(string); ok && status == "success" {
			if v, ok := result["country"].(string); ok {
				country = v
			}
			if v, ok := result["regionName"].(string); ok {
				region = v
			}
			if v, ok := result["city"].(string); ok {
				city = v
			}
			if v, ok := result["isp"].(string); ok {
				isp = v
			}
		}
	}

	return
}

// Load configuration from file
func loadConfig(configPath string) (*Config, error) {
	config := &Config{
		Connectors: make(map[string]ConnectorConfig),
		GeoIP: GeoIPConfig{
			Enabled: true,
			Service: "ipapi",
		},
		Debug: false,
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		return config, saveConfig(configPath, config)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, config)
	return config, err
}

// Save configuration to file
func saveConfig(configPath string, config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// Create notifiers from config
func createNotifiers(config *Config) []Notifier {
	var notifiers []Notifier

	for name, conn := range config.Connectors {
		if !conn.Enabled {
			continue
		}

		switch conn.Type {
		case "discord":
			if url, ok := conn.Settings["webhook_url"]; ok {
				notifier := &DiscordNotifier{
					WebhookURL: url,
					Username:   conn.Settings["username"],
					AvatarURL:  conn.Settings["avatar_url"],
				}
				if notifier.Username == "" {
					notifier.Username = "Fail2Ban"
				}
				notifiers = append(notifiers, notifier)
			}

		case "teams":
			if url, ok := conn.Settings["webhook_url"]; ok {
				notifiers = append(notifiers, &TeamsNotifier{WebhookURL: url})
			}

		case "slack":
			if url, ok := conn.Settings["webhook_url"]; ok {
				notifier := &SlackNotifier{
					WebhookURL: url,
					Channel:    conn.Settings["channel"],
					Username:   conn.Settings["username"],
					IconEmoji:  conn.Settings["icon_emoji"],
				}
				if notifier.Username == "" {
					notifier.Username = "fail2ban"
				}
				if notifier.IconEmoji == "" {
					notifier.IconEmoji = ":cop:"
				}
				notifiers = append(notifiers, notifier)
			}

		case "telegram":
			if token, ok := conn.Settings["bot_token"]; ok {
				if chatID, ok := conn.Settings["chat_id"]; ok {
					notifiers = append(notifiers, &TelegramNotifier{
						BotToken: token,
						ChatID:   chatID,
					})
				}
			}

		case "custom":
			if url, ok := conn.Settings["webhook_url"]; ok {
				if template, ok := conn.Settings["template"]; ok {
					headers := make(map[string]string)
					for key, value := range conn.Settings {
						if strings.HasPrefix(key, "header_") {
							headerName := strings.TrimPrefix(key, "header_")
							headers[headerName] = value
						}
					}
					notifiers = append(notifiers, &CustomNotifier{
						Name:       name,
						WebhookURL: url,
						Headers:    headers,
						Template:   template,
					})
				}
			}
		}
	}

	return notifiers
}

func main() {
	var (
		ip         = flag.String("ip", "", "IP address that was banned/unbanned")
		jail       = flag.String("jail", "", "Fail2ban jail name")
		action     = flag.String("action", "ban", "Action performed (ban/unban)")
		failures   = flag.Int("failures", 0, "Number of failures")
		configPath = flag.String("config", "/etc/fail2ban/fail2ban-notify.json", "Path to configuration file")
		initConfig = flag.Bool("init", false, "Initialize configuration file")
		debug      = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	if *initConfig {
		config := &Config{
			Connectors: map[string]ConnectorConfig{
				"discord": {
					Type:    "discord",
					Enabled: false,
					Settings: map[string]string{
						"webhook_url": "https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN",
						"username":    "Fail2Ban",
						"avatar_url":  "",
					},
				},
				"teams": {
					Type:    "teams",
					Enabled: false,
					Settings: map[string]string{
						"webhook_url": "https://your-tenant.webhook.office.com/webhookb2/YOUR_WEBHOOK_URL",
					},
				},
				"slack": {
					Type:    "slack",
					Enabled: false,
					Settings: map[string]string{
						"webhook_url": "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
						"channel":     "#security",
						"username":    "fail2ban",
						"icon_emoji":  ":cop:",
					},
				},
				"telegram": {
					Type:    "telegram",
					Enabled: false,
					Settings: map[string]string{
						"bot_token": "YOUR_BOT_TOKEN",
						"chat_id":   "YOUR_CHAT_ID",
					},
				},
				"custom": {
					Type:    "custom",
					Enabled: false,
					Settings: map[string]string{
						"webhook_url":     "https://your-custom-endpoint.com/webhook",
						"header_Content-Type": "application/json",
						"template":        `{"message": "IP {{.IP}} was {{.Action}}ed in jail {{.Jail}} at {{.Time}}"}`,
					},
				},
			},
			GeoIP: GeoIPConfig{
				Enabled: true,
				Service: "ipapi", // or "ipgeolocation"
				APIKey:  "",      // Required for ipgeolocation service
			},
			Debug: false,
		}

		if err := saveConfig(*configPath, config); err != nil {
			log.Fatalf("Failed to create config file: %v", err)
		}

		fmt.Printf("Configuration file created at: %s\n", *configPath)
		fmt.Println("Please edit the configuration file to enable and configure your notification services.")
		return
	}

	if *ip == "" || *jail == "" {
		flag.Usage()
		os.Exit(1)
	}

	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *debug {
		config.Debug = true
	}

	notifiers := createNotifiers(config)
	if len(notifiers) == 0 {
		log.Println("No notifiers configured or enabled")
		return
	}

	// Get geolocation info
	country, region, city, isp := getGeoIPInfo(*ip, config.GeoIP)

	data := NotificationData{
		IP:       *ip,
		Jail:     *jail,
		Action:   *action,
		Time:     time.Now(),
		Country:  country,
		Region:   region,
		City:     city,
		ISP:      isp,
		Failures: *failures,
	}

	if config.Debug {
		log.Printf("Notification data: %+v", data)
		log.Printf("Found %d enabled notifiers", len(notifiers))
	}

	// Send notifications
	for _, notifier := range notifiers {
		if config.Debug {
			log.Printf("Sending notification via %s", notifier.GetName())
		}

		if err := notifier.Send(data); err != nil {
			log.Printf("Failed to send %s notification: %v", notifier.GetName(), err)
		} else if config.Debug {
			log.Printf("Successfully sent %s notification", notifier.GetName())
		}
	}
}