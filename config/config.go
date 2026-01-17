package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Oracle        OracleConfig        `json:"oracle"`
	HealthFactor  HealthFactorConfig  `json:"health_factor"`
	Concentration ConcentrationConfig `json:"concentration"`
}

type OracleConfig struct {
	CheckIntervalSeconds int                   `json:"check_interval_seconds"`
	Stablecoin           OracleThresholdConfig `json:"stablecoin"`
	Volatile             OracleThresholdConfig `json:"volatile"`
}

type OracleThresholdConfig struct {
	ThresholdConfig
	DynamicCooldowns []DynamicCooldownConfig `json:"dynamic_cooldowns"`
}

type DynamicCooldownConfig struct {
	ThresholdPercent float64 `json:"threshold_percent"`
	CooldownSeconds  int     `json:"cooldown_seconds"`
}

type ThresholdConfig struct {
	WarningThresholdPercent  float64 `json:"warning_threshold_percent"`
	CriticalThresholdPercent float64 `json:"critical_threshold_percent"`
	MinValueChangePercent    float64 `json:"min_value_change_percent"`
	CooldownWarningMinutes   int     `json:"cooldown_warning_minutes"`
	CooldownCriticalMinutes  int     `json:"cooldown_critical_minutes"`
	ConsecutiveOKRequired    int     `json:"consecutive_ok_required"`
}

type HealthFactorConfig struct {
	CheckIntervalSeconds int            `json:"check_interval_seconds"`
	Position             PositionConfig `json:"position"`
	RiskyCountSpike      SpikeConfig    `json:"risky_count_spike"`
	AvgHFDrop            DropConfig     `json:"avg_hf_drop"`
	WithdrawalSpike      SpikeConfig    `json:"withdrawal_spike"`
	BorrowSpike          SpikeConfig    `json:"borrow_spike"`
}

type ConcentrationConfig struct {
	CheckIntervalSeconds int             `json:"check_interval_seconds"`
	WhaleSupply          ThresholdConfig `json:"whale_supply"`
	BorrowTop10          ThresholdConfig `json:"borrow_top10"`
	BorrowSingle         ThresholdConfig `json:"borrow_single"`
}

type PositionConfig struct {
	WarningThreshold        float64 `json:"warning_threshold"`
	CriticalThreshold       float64 `json:"critical_threshold"`
	MinValueChange          float64 `json:"min_value_change"`
	CooldownWarningMinutes  int     `json:"cooldown_warning_minutes"`
	CooldownCriticalMinutes int     `json:"cooldown_critical_minutes"`
	ConsecutiveOKRequired   int     `json:"consecutive_ok_required"`
	QueryLimit              int     `json:"query_limit"`
}

type SpikeConfig struct {
	WarningThresholdPercent  float64 `json:"warning_threshold_percent"`
	CriticalThresholdPercent float64 `json:"critical_threshold_percent"`
	MinValueChangePercent    float64 `json:"min_value_change_percent"`
	CooldownWarningMinutes   int     `json:"cooldown_warning_minutes"`
	CooldownCriticalMinutes  int     `json:"cooldown_critical_minutes"`
	ConsecutiveOKRequired    int     `json:"consecutive_ok_required"`
	CheckIntervalHours       int     `json:"check_interval_hours"`
}

type DropConfig struct {
	WarningThreshold        float64 `json:"warning_threshold"`
	CriticalThreshold       float64 `json:"critical_threshold"`
	MinValueChange          float64 `json:"min_value_change"`
	CooldownWarningMinutes  int     `json:"cooldown_warning_minutes"`
	CooldownCriticalMinutes int     `json:"cooldown_critical_minutes"`
	ConsecutiveOKRequired   int     `json:"consecutive_ok_required"`
	CheckIntervalHours      int     `json:"check_interval_hours"`
}

// Helper methods
func (t ThresholdConfig) CooldownWarning() time.Duration {
	return time.Duration(t.CooldownWarningMinutes) * time.Minute
}

func (t ThresholdConfig) CooldownCritical() time.Duration {
	return time.Duration(t.CooldownCriticalMinutes) * time.Minute
}

func (p PositionConfig) CooldownWarning() time.Duration {
	return time.Duration(p.CooldownWarningMinutes) * time.Minute
}

func (p PositionConfig) CooldownCritical() time.Duration {
	return time.Duration(p.CooldownCriticalMinutes) * time.Minute
}

func (s SpikeConfig) CooldownWarning() time.Duration {
	return time.Duration(s.CooldownWarningMinutes) * time.Minute
}

func (s SpikeConfig) CooldownCritical() time.Duration {
	return time.Duration(s.CooldownCriticalMinutes) * time.Minute
}

func (d DropConfig) CooldownWarning() time.Duration {
	return time.Duration(d.CooldownWarningMinutes) * time.Minute
}

func (d DropConfig) CooldownCritical() time.Duration {
	return time.Duration(d.CooldownCriticalMinutes) * time.Minute
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

func LoadOrDefault(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		fmt.Printf("warning: could not load config from %s: %v, using defaults\n", path, err)
		return DefaultConfig()
	}
	return cfg
}

func DefaultConfig() *Config {
	return &Config{
		Oracle: OracleConfig{
			CheckIntervalSeconds: 120,
			Stablecoin: OracleThresholdConfig{
				ThresholdConfig: ThresholdConfig{
					WarningThresholdPercent:  1.0,
					CriticalThresholdPercent: 2.0,
					MinValueChangePercent:    0.2,
					CooldownWarningMinutes:   30,
					CooldownCriticalMinutes:  5,
					ConsecutiveOKRequired:    3,
				},
				DynamicCooldowns: []DynamicCooldownConfig{
					{ThresholdPercent: 10.0, CooldownSeconds: 10},
					{ThresholdPercent: 5.0, CooldownSeconds: 30},
				},
			},
			Volatile: OracleThresholdConfig{
				ThresholdConfig: ThresholdConfig{
					WarningThresholdPercent:  3.0,
					CriticalThresholdPercent: 5.0,
					MinValueChangePercent:    1.0,
					CooldownWarningMinutes:   30,
					CooldownCriticalMinutes:  5,
					ConsecutiveOKRequired:    1,
				},
				DynamicCooldowns: []DynamicCooldownConfig{
					{ThresholdPercent: 20.0, CooldownSeconds: 10},
					{ThresholdPercent: 10.0, CooldownSeconds: 30},
				},
			},
		},
		HealthFactor: HealthFactorConfig{
			CheckIntervalSeconds: 300,
			Position: PositionConfig{
				WarningThreshold:        1.5,
				CriticalThreshold:       1.02,
				MinValueChange:          0.05,
				CooldownWarningMinutes:  30,
				CooldownCriticalMinutes: 10,
				ConsecutiveOKRequired:   3,
				QueryLimit:              100,
			},
			RiskyCountSpike: SpikeConfig{
				WarningThresholdPercent:  25.0,
				CriticalThresholdPercent: 50.0,
				MinValueChangePercent:    5.0,
				CooldownWarningMinutes:   60,
				CooldownCriticalMinutes:  30,
				ConsecutiveOKRequired:    2,
				CheckIntervalHours:       24,
			},
			AvgHFDrop: DropConfig{
				WarningThreshold:        0.1,
				CriticalThreshold:       0.2,
				MinValueChange:          0.02,
				CooldownWarningMinutes:  30,
				CooldownCriticalMinutes: 15,
				ConsecutiveOKRequired:   2,
				CheckIntervalHours:      1,
			},
			WithdrawalSpike: SpikeConfig{
				WarningThresholdPercent:  10.0,
				CriticalThresholdPercent: 20.0,
				MinValueChangePercent:    2.0,
				CooldownWarningMinutes:   60,
				CooldownCriticalMinutes:  30,
				ConsecutiveOKRequired:    2,
				CheckIntervalHours:       24,
			},
			BorrowSpike: SpikeConfig{
				WarningThresholdPercent:  10.0,
				CriticalThresholdPercent: 20.0,
				MinValueChangePercent:    2.0,
				CooldownWarningMinutes:   60,
				CooldownCriticalMinutes:  30,
				ConsecutiveOKRequired:    2,
				CheckIntervalHours:       24,
			},
		},
		Concentration: ConcentrationConfig{
			CheckIntervalSeconds: 600,
			WhaleSupply: ThresholdConfig{
				WarningThresholdPercent:  10.0,
				CriticalThresholdPercent: 20.0,
				MinValueChangePercent:    1.0,
				CooldownWarningMinutes:   60,
				CooldownCriticalMinutes:  30,
				ConsecutiveOKRequired:    3,
			},
			BorrowTop10: ThresholdConfig{
				WarningThresholdPercent:  80.0,
				CriticalThresholdPercent: 90.0,
				MinValueChangePercent:    2.0,
				CooldownWarningMinutes:   60,
				CooldownCriticalMinutes:  30,
				ConsecutiveOKRequired:    3,
			},
			BorrowSingle: ThresholdConfig{
				WarningThresholdPercent:  40.0,
				CriticalThresholdPercent: 50.0,
				MinValueChangePercent:    2.0,
				CooldownWarningMinutes:   60,
				CooldownCriticalMinutes:  30,
				ConsecutiveOKRequired:    3,
			},
		},
	}
}
