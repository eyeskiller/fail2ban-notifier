package setup

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/eyeskiller/fail2ban-notifier/internal/config"
	"github.com/eyeskiller/fail2ban-notifier/internal/connectors"
)

const (
	actionConfigPath = "/etc/fail2ban/action.d/notify.conf"
	jailConfigDir    = "/etc/fail2ban/jail.d"
	fail2banConfDir  = "/etc/fail2ban"
	defaultConfigDir = "/etc/fail2ban"
)

type SetupWizard struct {
	configPath string
	cfg        *config.Config
	logger     *log.Logger
	reader     *bufio.Reader
}

func NewSetupWizard(configPath string, cfg *config.Config, logger *log.Logger) *SetupWizard {
	return &SetupWizard{
		configPath: configPath,
		cfg:        cfg,
		logger:     logger,
		reader:     bufio.NewReader(os.Stdin),
	}
}

func (w *SetupWizard) Run() error {
	w.printBanner()

	if err := w.checkPrerequisites(); err != nil {
		return err
	}

	if err := w.installActionConfig(); err != nil {
		w.logger.Printf("Warning: Failed to install action config: %v", err)
	}

	if err := w.createAppConfig(); err != nil {
		return err
	}

	if err := w.configureConnectors(); err != nil {
		return err
	}

	if err := w.testConfiguredConnectors(); err != nil {
		w.logger.Printf("Warning: Some connector tests failed: %v", err)
	}

	w.integrateJails()

	w.printSummary()
	return nil
}

func (w *SetupWizard) printBanner() {
	fmt.Print(`
╔══════════════════════════════════════════════════╗
║        Fail2Ban Notifier Setup Wizard            ║
║   Automated notification setup for Fail2Ban      ║
╚══════════════════════════════════════════════════╝
`)
}

func (w *SetupWizard) checkPrerequisites() error {
	fmt.Println("▶ Checking prerequisites...")

	if os.Geteuid() != 0 {
		fmt.Println("  ⚠  Not running as root. Some operations may fail.")
		fmt.Println("     Consider running with: sudo fail2ban-notify -setup")
	}

	if _, err := os.Stat(fail2banConfDir); os.IsNotExist(err) {
		return fmt.Errorf("fail2ban configuration directory not found at %s. Is fail2ban installed?", fail2banConfDir)
	}
	fmt.Println("  ✅  Fail2Ban configuration directory found")

	if _, err := exec.LookPath("fail2ban-client"); err == nil {
		cmd := exec.Command("fail2ban-client", "status")
		if output, err := cmd.Output(); err == nil {
			fmt.Printf("  ✅  Fail2Ban is running\n")
			_ = output
		} else {
			fmt.Println("  ⚠  fail2ban-client found but could not connect. Is the service running?")
			fmt.Printf("     Error: %s\n", strings.TrimSpace(string(output)))
		}
	} else {
		fmt.Println("  ⚠  fail2ban-client not found in PATH. Is fail2ban installed?")
	}

	fmt.Println()
	return nil
}

func (w *SetupWizard) installActionConfig() error {
	fmt.Println("▶ Installing Fail2Ban action configuration...")

	if _, err := os.Stat(actionConfigPath); err == nil {
		overwrite := w.promptConfirm(fmt.Sprintf("Action config already exists at %s. Overwrite?", actionConfigPath), false)
		if !overwrite {
			fmt.Println("  ⏭  Skipping action config install")
			fmt.Println()
			return nil
		}
	}

	actionDir := filepath.Dir(actionConfigPath)
	if err := os.MkdirAll(actionDir, 0750); err != nil {
		return fmt.Errorf("failed to create action directory %s: %w", actionDir, err)
	}

	content := `# Fail2Ban notification action configuration
# Installed by fail2ban-notify setup wizard

[INCLUDES]

before = iptables-common.conf

[Definition]

actionstart =

actionstop =

actioncheck =

actionban = /usr/local/bin/fail2ban-notify -ip="<ip>" -jail="<name>" -action="ban" -failures="<failures>"

actionunban = /usr/local/bin/fail2ban-notify -ip="<ip>" -jail="<name>" -action="unban" -failures="<failures>"

[Init]

name = default
`

	if err := os.WriteFile(actionConfigPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write action config: %w", err)
	}
	fmt.Printf("  ✅  Action config installed to %s\n", actionConfigPath)
	fmt.Println()
	return nil
}

func (w *SetupWizard) createAppConfig() error {
	fmt.Println("▶ Creating application configuration...")

	if _, err := os.Stat(w.configPath); err == nil {
		fmt.Printf("  ✅  Config already exists at %s\n", w.configPath)
		fmt.Println()
		return nil
	}

	sampleConfig := config.CreateSampleConfig()

	manager := connectors.NewManager(w.cfg, w.logger)
	if discovered, err := manager.DiscoverConnectors(); err == nil {
		for _, c := range discovered {
			conn := c
			sampleConfig.AddConnector(&conn)
		}
		if len(discovered) > 0 {
			fmt.Printf("  📁  Discovered %d connectors\n", len(discovered))
		}
	}

	if err := config.SaveConfig(w.configPath, sampleConfig); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	*w.cfg = *sampleConfig
	fmt.Printf("  ✅  Config created at %s\n", w.configPath)
	fmt.Println()
	return nil
}

func (w *SetupWizard) configureConnectors() error {
	fmt.Println("▶ Connector Configuration")
	fmt.Println("   Available connectors:")

	manager := connectors.NewManager(w.cfg, w.logger)
	statuses := manager.GetConnectorStatus()

	var connectorNames []string
	for name := range statuses {
		connectorNames = append(connectorNames, name)
	}

	for i, name := range connectorNames {
		s := statuses[name]
		fmt.Printf("   %d. %s [%s] - %s\n", i+1, name, s.Type, s.Description)
	}
	fmt.Println()

	configure := w.promptConfirm("Would you like to configure a notification connector?", true)
	if !configure {
		fmt.Println("  ⏭  Skipping connector configuration")
		fmt.Println()
		return nil
	}

	for {
		fmt.Print("Enter the number or name of the connector to configure (or 'done' to finish): ")
		input, _ := w.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if strings.ToLower(input) == "done" || strings.ToLower(input) == "quit" || strings.ToLower(input) == "exit" {
			break
		}

		selectedName := ""
		if idx := parseIndex(input); idx >= 0 && idx < len(connectorNames) {
			selectedName = connectorNames[idx]
		} else {
			for _, name := range connectorNames {
				if strings.EqualFold(name, input) {
					selectedName = name
					break
				}
			}
		}

		if selectedName == "" {
			fmt.Printf("  ❌  Unknown connector: %s\n", input)
			continue
		}

		w.configureSingleConnector(selectedName, statuses[selectedName])
	}

	fmt.Println()
	return nil
}

func (w *SetupWizard) configureSingleConnector(name string, status connectors.ConnectorStatus) {
	fmt.Printf("\n--- Configuring %s ---\n", name)

	for i, c := range w.cfg.Connectors {
		if c.Name == name {
			updated := w.promptConnectorSettings(&c)
			w.cfg.Connectors[i] = updated
			if err := config.SaveConfig(w.configPath, w.cfg); err != nil {
				w.logger.Printf("Warning: Failed to save config: %v", err)
			}
			fmt.Printf("  ✅  %s configured and saved\n", name)
			return
		}
	}

	fmt.Printf("  ⚠  Connector %s not found in config. Creating new entry.\n", name)
	newConn := config.ConnectorConfig{
		Name:    name,
		Type:    status.Type,
		Enabled: true,
		Path:    status.Path,
		Settings: map[string]string{},
		Timeout: 30,
	}
	updated := w.promptConnectorSettings(&newConn)
	w.cfg.AddConnector(&updated)
	if err := config.SaveConfig(w.configPath, w.cfg); err != nil {
		w.logger.Printf("Warning: Failed to save config: %v", err)
	}
	fmt.Printf("  ✅  %s configured and saved\n", name)
}

var connectorSettings = map[string][]SettingDef{
	"discord": {
		{Key: "DISCORD_WEBHOOK_URL", Label: "Discord Webhook URL", Required: true, Secret: true},
		{Key: "DISCORD_USERNAME", Label: "Bot username (leave empty for default)", Required: false},
		{Key: "DISCORD_AVATAR_URL", Label: "Avatar image URL (leave empty for default)", Required: false},
	},
	"slack": {
		{Key: "SLACK_WEBHOOK_URL", Label: "Slack Webhook URL", Required: true, Secret: true},
		{Key: "SLACK_CHANNEL", Label: "Channel (e.g., #security)", Required: false},
		{Key: "SLACK_USERNAME", Label: "Bot username (leave empty for default)", Required: false},
	},
	"teams": {
		{Key: "TEAMS_WEBHOOK_URL", Label: "Teams Webhook URL", Required: true, Secret: true},
	},
	"telegram": {
		{Key: "TELEGRAM_BOT_TOKEN", Label: "Bot Token (from @BotFather)", Required: true, Secret: true},
		{Key: "TELEGRAM_CHAT_ID", Label: "Chat ID", Required: true},
	},
	"email": {
		{Key: "EMAIL_SMTP_SERVER", Label: "SMTP Server", Required: true},
		{Key: "EMAIL_SMTP_PORT", Label: "SMTP Port", Required: false},
		{Key: "EMAIL_SMTP_USER", Label: "SMTP Username", Required: false},
		{Key: "EMAIL_SMTP_PASSWORD", Label: "SMTP Password", Required: false, Secret: true},
		{Key: "EMAIL_SMTP_TLS", Label: "Use TLS (true/false)", Required: false},
		{Key: "EMAIL_FROM", Label: "From address", Required: false},
		{Key: "EMAIL_TO", Label: "To address", Required: true},
	},
	"pagerduty": {
		{Key: "PAGERDUTY_ROUTING_KEY", Label: "PagerDuty Routing Key (Integration Key)", Required: true, Secret: true},
	},
	"webhook": {
		{Key: "url", Label: "Webhook URL", Required: true, Secret: true},
	},
}

type SettingDef struct {
	Key      string
	Label    string
	Required bool
	Secret   bool
}

func (w *SetupWizard) promptConnectorSettings(c *config.ConnectorConfig) config.ConnectorConfig {
	enabled := w.promptConfirm(fmt.Sprintf("Enable %s?", c.Name), true)
	c.Enabled = enabled

	if !enabled {
		return *c
	}

	settings, ok := connectorSettings[c.Name]
	if !ok {
		fmt.Printf("  ⚠  No predefined settings for %s. You can edit the config manually.\n", c.Name)
		return *c
	}

	if c.Settings == nil {
		c.Settings = make(map[string]string)
	}

	for _, setting := range settings {
		currentVal := c.Settings[setting.Key]
		promptMsg := fmt.Sprintf("  %s", setting.Label)
		if setting.Required {
			promptMsg += " [required]"
		}
		if currentVal != "" {
			promptMsg += fmt.Sprintf(" (current: %s)", maskString(currentVal))
		}
		promptMsg += ": "

		val := w.promptInput(promptMsg, currentVal, setting.Secret)

		if setting.Required && val == "" {
			fmt.Println("  ❌  This setting is required. Please provide a value.")
			val = w.promptInput(promptMsg, "", setting.Secret)
			if val == "" {
				fmt.Println("  ⚠  Skipping this setting. You can edit it later in the config file.")
				continue
			}
		}

		if val != "" {
			c.Settings[setting.Key] = val
		}
	}

	return *c
}

func (w *SetupWizard) testConfiguredConnectors() error {
	fmt.Println("▶ Testing configured connectors...")

	manager := connectors.NewManager(w.cfg, w.logger)
	enabled := w.cfg.GetEnabledConnectors()

	if len(enabled) == 0 {
		fmt.Println("  ⏭  No enabled connectors to test")
		fmt.Println()
		return nil
	}

	runTest := w.promptConfirm(fmt.Sprintf("Test %d enabled connector(s)?", len(enabled)), true)
	if !runTest {
		fmt.Println("  ⏭  Skipping connector tests")
		fmt.Println()
		return nil
	}

	var lastErr error
	for _, c := range enabled {
		fmt.Printf("  Testing %s...", c.Name)
		if err := manager.TestConnector(c.Name, nil); err != nil {
			fmt.Printf(" ❌\n  Error: %v\n", err)
			lastErr = err
		} else {
			fmt.Println(" ✅")
		}
	}

	fmt.Println()
	return lastErr
}

func (w *SetupWizard) integrateJails() {
	fmt.Println("▶ Fail2Ban Jail Integration")

	integrate := w.promptConfirm("Would you like to add the notify action to specific jails?", true)
	if !integrate {
		fmt.Println("  ⏭  Skipping jail integration")
		fmt.Println()
		return
	}

	jails := w.discoverJails()
	if len(jails) == 0 {
		fmt.Println("  ⚠  No jails detected. You can add 'action = notify' manually to your jails.")
		fmt.Println()
		return
	}

	fmt.Println("   Detected jails:")
	for i, name := range jails {
		fmt.Printf("   %d. %s\n", i+1, name)
	}
	fmt.Println()

	var selectedJails []string
	for {
		fmt.Print("Enter jail numbers or names to integrate (comma/space separated, or 'all' for all, 'done' to finish): ")
		input, _ := w.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if strings.ToLower(input) == "done" || strings.ToLower(input) == "exit" {
			break
		}
		if strings.ToLower(input) == "all" {
			selectedJails = jails
			break
		}

		parts := strings.FieldsFunc(input, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t'
		})
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if idx := parseIndex(part); idx >= 0 && idx < len(jails) {
				selectedJails = append(selectedJails, jails[idx])
			} else {
				for _, name := range jails {
					if strings.EqualFold(name, part) && !contains(selectedJails, name) {
						selectedJails = append(selectedJails, name)
					}
				}
			}
		}
		if len(selectedJails) > 0 {
			break
		}
	}

	if len(selectedJails) == 0 {
		fmt.Println("  ⏭  No jails selected")
		fmt.Println()
		return
	}

	if err := w.writeJailConfig(selectedJails); err != nil {
		w.logger.Printf("  ❌  Failed to write jail config: %v", err)
	} else {
		fmt.Printf("  ✅  Notify action added to %d jail(s)\n", len(selectedJails))
	}

	fmt.Println()
}

func (w *SetupWizard) discoverJails() []string {
	var jails []string

	if _, err := exec.LookPath("fail2ban-client"); err == nil {
		cmd := exec.Command("fail2ban-client", "status")
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "- ") {
					name := strings.TrimPrefix(line, "- ")
					name = strings.TrimSpace(name)
					if name != "" && name != "Jail list" && !strings.Contains(name, ":") {
						jails = append(jails, name)
					}
				}
			}
			if len(jails) > 0 {
				return jails
			}
		}
	}

	files, err := filepath.Glob(filepath.Join(fail2banConfDir, "jail.d/*.conf"))
	if err != nil {
		return jails
	}
	for _, f := range files {
		base := filepath.Base(f)
		name := strings.TrimSuffix(base, ".conf")
		if name != "notify" {
			jails = append(jails, name)
		}
	}

	files, err = filepath.Glob(filepath.Join(fail2banConfDir, "jail.d/*.local"))
	if err == nil {
		for _, f := range files {
			base := filepath.Base(f)
			name := strings.TrimSuffix(base, ".local")
			if name != "notify" {
				jails = append(jails, name)
			}
		}
	}

	if _, err := os.Stat("/etc/fail2ban/jail.conf"); err == nil {
		if !contains(jails, "sshd") {
			jails = append(jails, "sshd")
		}
	}

	return jails
}

func (w *SetupWizard) writeJailConfig(jails []string) error {
	if err := os.MkdirAll(jailConfigDir, 0750); err != nil {
		return fmt.Errorf("failed to create jail.d directory: %w", err)
	}

	path := filepath.Join(jailConfigDir, "notify.local")
	exists := true
	if _, err := os.Stat(path); os.IsNotExist(err) {
		exists = false
	}

	overwrite := true
	if exists {
		overwrite = w.promptConfirm(fmt.Sprintf("Jail config already exists at %s. Overwrite?", path), false)
	}

	if !overwrite {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("# Fail2Ban notify action configuration\n")
	sb.WriteString("# Installed by fail2ban-notify setup wizard\n")
	sb.WriteString("# To enable notifications for additional jails, add: action = notify\n\n")

	for _, jail := range jails {
		sb.WriteString(fmt.Sprintf("[%s]\naction = notify\n\n", jail))
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("failed to write jail config: %w", err)
	}

	return nil
}

func (w *SetupWizard) printSummary() {
	fmt.Print(`
╔══════════════════════════════════════════════════╗
║                Setup Complete!                    ║
╚══════════════════════════════════════════════════╝
`)
	fmt.Printf("  Configuration: %s\n", w.configPath)
	fmt.Printf("  Action config: %s\n", actionConfigPath)
	enabled := w.cfg.GetEnabledConnectors()
	fmt.Printf("  Enabled connectors: %d\n", len(enabled))
	for _, c := range enabled {
		fmt.Printf("    - %s\n", c.Name)
	}

	fmt.Print(`
  Next steps:
   1. Reload Fail2Ban to apply jail changes:
      sudo fail2ban-client reload

   2. Check connector status:
      sudo fail2ban-notify -status

   3. Test a connector:
      sudo fail2ban-notify -test <connector-name>

   4. View logs for troubleshooting:
      sudo journalctl -u fail2ban -f

   5. For help: fail2ban-notify -setup to run this wizard again
`)
}
