package alerts

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// Severity levels for alerts
type Severity string

const (
	SeverityOK       Severity = "OK"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
)

// AlertKey uniquely identifies an alert instance
type AlertKey struct {
	Job    string // e.g. "oracle_deviation"
	Entity string // e.g. "WETH", or user address
	Metric string // e.g. "price_deviation", "health_factor"
}

// AlertState tracks the current state of an alert
type AlertState struct {
	Severity       Severity
	LastSent       time.Time
	FirstTriggered time.Time
	LastValue      float64
	LastMessage    string
	ConsecutiveOK  int // for hysteresis
}

// AlertPolicy defines the behavior for a specific alert type
type AlertPolicy struct {
	// Minimum % change in metric required to re-send an alert at same severity
	MinValueChange float64

	// Cooldowns per severity for repeated alerts
	CooldownWarning  time.Duration
	CooldownCritical time.Duration

	// Dynamic cooldowns based on value thresholds
	// Format: [[threshold1, cooldown1], [threshold2, cooldown2], ...]
	// Sorted by threshold descending (highest first)
	DynamicCooldowns []DynamicCooldown

	// Optional periodic reminder while still bad
	ReminderInterval time.Duration

	// Threshold to trigger the alert
	TriggerThreshold float64

	// Number of consecutive OK readings before clearing
	ConsecutiveOKRequired int
}

type DynamicCooldown struct {
	Threshold float64       // Value threshold (e.g., 20 for 20%)
	Cooldown  time.Duration // Cooldown when value >= threshold
}

// Manager handles stateful alert lifecycle management
type Manager struct {
	mu       sync.RWMutex
	states   map[AlertKey]*AlertState
	policies map[string]AlertPolicy // keyed by "job:metric"
	service  *Service
	clock    func() time.Time // for testability
}

// NewManager creates a new alert manager
func NewManager(service *Service) *Manager {
	return &Manager{
		states:   make(map[AlertKey]*AlertState),
		policies: make(map[string]AlertPolicy),
		service:  service,
		clock:    time.Now,
	}
}

// RegisterPolicy registers an alert policy for a job:metric combination
func (m *Manager) RegisterPolicy(job, metric string, policy AlertPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s", job, metric)
	m.policies[key] = policy
}

// alertAction represents what action to take after evaluating an observation
type alertAction struct {
	shouldSend      bool
	message         string
	isBusinessAlert bool
	slackMessage    string
	newState        *AlertState
	deleteState     bool
}

// Observe processes a new observation and decides whether to send an alert
// slackMessage is optional - if provided, it will be sent to Slack alongside Telegram for business alerts
func (m *Manager) Observe(
	ctx context.Context,
	key AlertKey,
	severity Severity,
	value float64,
	summary string,
	details string,
	isBusinessAlert bool,
	slackMessage string,
) error {
	// Determine action under lock, then release before network I/O
	action := m.evaluateObservation(key, severity, value, summary, details, isBusinessAlert, slackMessage)

	// No action needed
	if !action.shouldSend && action.newState == nil && !action.deleteState {
		return nil
	}

	// Send alert outside of lock to prevent blocking
	if action.shouldSend {
		if err := m.sendAlert(ctx, action.message, action.isBusinessAlert, action.slackMessage); err != nil {
			return err
		}
	}

	// Update state after successful send (or if just updating state without send)
	if action.newState != nil || action.deleteState {
		m.mu.Lock()
		if action.deleteState {
			delete(m.states, key)
		} else if action.newState != nil {
			m.states[key] = action.newState
		}
		m.mu.Unlock()
	}

	return nil
}

// evaluateObservation determines what action to take for an observation (called under lock)
func (m *Manager) evaluateObservation(
	key AlertKey,
	severity Severity,
	value float64,
	summary string,
	details string,
	isBusinessAlert bool,
	slackMessage string,
) alertAction {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.clock()
	state, exists := m.states[key]
	policyKey := fmt.Sprintf("%s:%s", key.Job, key.Metric)
	policy, hasPolicy := m.policies[policyKey]

	// Use default policy if none registered
	if !hasPolicy {
		policy = AlertPolicy{
			MinValueChange:        10.0,
			CooldownWarning:       15 * time.Minute,
			CooldownCritical:      5 * time.Minute,
			ReminderInterval:      60 * time.Minute,
			ConsecutiveOKRequired: 2,
		}
	}

	// 1. Handle OK severity (recovery or clear)
	if severity == SeverityOK {
		if !exists {
			return alertAction{} // nothing to clear
		}

		state.ConsecutiveOK++

		// Need multiple consecutive OK readings for hysteresis
		if state.ConsecutiveOK >= policy.ConsecutiveOKRequired && state.Severity != SeverityOK {
			// Silently clear the alert without sending a recovery notification
			return alertAction{deleteState: true}
		}
		// Update state with incremented ConsecutiveOK
		m.states[key] = state
		return alertAction{}
	}

	// Reset consecutive OK counter since we have a non-OK reading
	if exists {
		state.ConsecutiveOK = 0
	}

	// 2. New incident (no previous state or was OK)
	if !exists || state.Severity == SeverityOK {
		msg := m.formatNewIncidentMessage(key, severity, value, summary, details)
		return alertAction{
			shouldSend:      true,
			message:         msg,
			isBusinessAlert: isBusinessAlert,
			slackMessage:    slackMessage,
			newState: &AlertState{
				Severity:       severity,
				LastSent:       now,
				FirstTriggered: now,
				LastValue:      value,
				LastMessage:    msg,
				ConsecutiveOK:  0,
			},
		}
	}

	// 3. Escalation (WARNING -> CRITICAL)
	if severityLevel(severity) > severityLevel(state.Severity) {
		msg := m.formatEscalationMessage(key, state, severity, value, summary, details)
		return alertAction{
			shouldSend:      true,
			message:         msg,
			isBusinessAlert: isBusinessAlert,
			slackMessage:    slackMessage,
			newState: &AlertState{
				Severity:       severity,
				LastSent:       now,
				FirstTriggered: state.FirstTriggered,
				LastValue:      value,
				LastMessage:    msg,
				ConsecutiveOK:  0,
			},
		}
	}

	// 4. De-escalation (CRITICAL -> WARNING)
	if severityLevel(severity) < severityLevel(state.Severity) {
		// De-escalation goes to developer channel only, not business (no Slack)
		msg := m.formatDeescalationMessage(key, state, severity, value, summary, details)
		return alertAction{
			shouldSend:      true,
			message:         msg,
			isBusinessAlert: false,
			slackMessage:    "",
			newState: &AlertState{
				Severity:       severity,
				LastSent:       now,
				FirstTriggered: state.FirstTriggered,
				LastValue:      value,
				LastMessage:    msg,
				ConsecutiveOK:  0,
			},
		}
	}

	// 5. Same severity: check cooldown and value change
	cooldown := m.calculateCooldown(policy, severity, value)

	timeSinceLastSent := now.Sub(state.LastSent)
	timeSinceFirstTriggered := now.Sub(state.FirstTriggered)

	// Check for periodic reminder
	// Reminders only go to developer channel, and only for CRITICAL issues (no Slack)
	if policy.ReminderInterval > 0 &&
		timeSinceFirstTriggered >= policy.ReminderInterval &&
		timeSinceLastSent >= policy.ReminderInterval &&
		severity == SeverityCritical {
		msg := m.formatNewIncidentMessage(key, severity, value, summary, details)
		return alertAction{
			shouldSend:      true,
			message:         msg,
			isBusinessAlert: false,
			slackMessage:    "",
			newState: &AlertState{
				Severity:       severity,
				LastSent:       now,
				FirstTriggered: state.FirstTriggered,
				LastValue:      value,
				LastMessage:    msg,
				ConsecutiveOK:  0,
			},
		}
	}

	// Still in cooldown period
	if timeSinceLastSent < cooldown {
		return alertAction{}
	}

	// Check if value changed significantly
	var percentChange float64
	if state.LastValue != 0 {
		percentChange = math.Abs((value - state.LastValue) / state.LastValue * 100)
	} else if value != 0 {
		percentChange = 100.0 // 0 to any non-zero value is considered 100% change
	}
	if percentChange < policy.MinValueChange {
		return alertAction{} // minor fluctuation, don't resend
	}

	// Significant change after cooldown
	msg := m.formatUpdateMessage(key, state, severity, value, summary, details)
	// Updates for CRITICAL go to business, WARNING updates go to developer only
	sendToBusiness := isBusinessAlert && severity == SeverityCritical
	slackForUpdate := ""
	if sendToBusiness {
		slackForUpdate = slackMessage
	}

	return alertAction{
		shouldSend:      true,
		message:         msg,
		isBusinessAlert: sendToBusiness,
		slackMessage:    slackForUpdate,
		newState: &AlertState{
			Severity:       severity,
			LastSent:       now,
			FirstTriggered: state.FirstTriggered,
			LastValue:      value,
			LastMessage:    msg,
			ConsecutiveOK:  0,
		},
	}
}

// GetActiveIncidents returns all currently active incidents
func (m *Manager) GetActiveIncidents() map[AlertKey]AlertState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[AlertKey]AlertState)
	for k, v := range m.states {
		if v.Severity != SeverityOK {
			result[k] = *v
		}
	}
	return result
}

// ClearAll clears all alert states (useful for testing)
func (m *Manager) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states = make(map[AlertKey]*AlertState)
}

func (m *Manager) calculateCooldown(policy AlertPolicy, severity Severity, value float64) time.Duration {
	// Check for dynamic cooldowns first
	if len(policy.DynamicCooldowns) > 0 {
		// DynamicCooldowns should be sorted by threshold descending (highest first)
		for _, dc := range policy.DynamicCooldowns {
			if value >= dc.Threshold {
				return dc.Cooldown
			}
		}
	}

	// Fall back to severity-based cooldowns
	if severity == SeverityCritical {
		return policy.CooldownCritical
	}
	return policy.CooldownWarning
}

func (m *Manager) sendAlert(ctx context.Context, message string, isBusinessAlert bool, slackMessage string) error {
	if isBusinessAlert {
		if err := m.service.SendBusinessAlert(ctx, message); err != nil {
			return err
		}
		// Also send to Slack for business alerts if slackMessage is provided
		if slackMessage != "" {
			if err := m.service.SendSlackAlert(ctx, slackMessage); err != nil {
				// Log but don't fail - Telegram is primary
				fmt.Printf("[alerts] slack alert failed: %v\n", err)
			}
		}
		// Also send business alerts to developer channel for visibility
		if err := m.service.SendDeveloperAlert(ctx, message); err != nil {
			// Log but don't fail - business channel is primary
			fmt.Printf("[alerts] developer alert failed: %v\n", err)
		}
		return nil
	}
	return m.service.SendDeveloperAlert(ctx, message)
}

func severityLevel(s Severity) int {
	switch s {
	case SeverityOK:
		return 0
	case SeverityWarning:
		return 1
	case SeverityCritical:
		return 2
	default:
		return 0
	}
}

// Message formatting functions

func (m *Manager) getAlertTitle(job, metric string) string {
	// Use metric-based lookup since job names vary (e.g., oracle_base, oracle_optimism)
	metricTitles := map[string]string{
		"price_deviation_stable":   "STABLECOIN DEPEG ALERT",
		"price_deviation_volatile": "ORACLE PRICE DEVIATION",
		"system_health":            "ORACLE SYSTEM HEALTH",
		"data_staleness":           "DATA STALE",
		"token_error":              "TOKEN PRICE ERROR",
		"position_risk":            "LOW HEALTH FACTOR POSITION",
		"risky_count_spike":        "RISKY POSITIONS SPIKE",
		"avg_hf_drop":              "AVERAGE HEALTH FACTOR DROP",
		"withdrawal_spike":         "WITHDRAWAL SPIKE ALERT",
		"borrow_spike":             "BORROW SPIKE ALERT",
		"whale_supply":             "WHALE POSITION ALERT",
		"borrow_top10":             "BORROW CONCENTRATION - TOP 10",
		"borrow_single":            "BORROW CONCENTRATION - SINGLE WALLET",
	}

	if title, ok := metricTitles[metric]; ok {
		return title
	}
	return strings.ToUpper(strings.ReplaceAll(metric, "_", " "))
}

func (m *Manager) formatNewIncidentMessage(key AlertKey, severity Severity, value float64, summary, details string) string {
	title := m.getAlertTitle(key.Job, key.Metric)
	return fmt.Sprintf(
		"🚨 %s\n\n%s",
		title,
		details,
	)
}

func (m *Manager) formatEscalationMessage(key AlertKey, state *AlertState, newSeverity Severity, value float64, summary, details string) string {
	title := m.getAlertTitle(key.Job, key.Metric)
	return fmt.Sprintf(
		"🚨 %s\n\n%s",
		title,
		details,
	)
}

func (m *Manager) formatDeescalationMessage(key AlertKey, state *AlertState, newSeverity Severity, value float64, summary, details string) string {
	title := m.getAlertTitle(key.Job, key.Metric)
	return fmt.Sprintf(
		"✅ %s\n\n%s",
		title,
		details,
	)
}

func (m *Manager) formatUpdateMessage(key AlertKey, state *AlertState, severity Severity, value float64, summary, details string) string {
	title := m.getAlertTitle(key.Job, key.Metric)
	return fmt.Sprintf(
		"🚨 %s\n\n%s",
		title,
		details,
	)
}
