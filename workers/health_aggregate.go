package workers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"

	"github.com/0x0Glitch/alerts"
)

// HealthAggregateJob monitors systemic health factor metrics
type HealthAggregateJob struct {
	db                  *sql.DB
	alertManager        *alerts.Manager
	lastAvgHealthFactor float64
	lastRiskyCountCheck time.Time
	last24hRiskyCount   int
	last24hCheckTime    time.Time
	last24hTotalSupply  float64
	last24hTotalBorrow  float64
	last24hSupplyTime   time.Time // Separate timestamp for supply tracking
	last24hBorrowTime   time.Time // Separate timestamp for borrow tracking
}

type aggregateMetrics struct {
	TotalPositions     int
	RiskyPositions     int
	AvgHealthFactor    float64
	WeightedAvgHF      float64
	TotalCollateralUSD float64
	TotalBorrowUSD     float64
}

// NewHealthAggregateJob creates a new aggregate health monitoring job
func NewHealthAggregateJob(databaseURL string, alertManager *alerts.Manager) (*HealthAggregateJob, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL not configured")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Register policies for aggregate health alerts
	alertManager.RegisterPolicy("health_aggregate", "risky_count_spike", alerts.AlertPolicy{
		MinValueChange:        5.0, // 5% change in risky count
		CooldownWarning:       1 * time.Hour,
		CooldownCritical:      30 * time.Minute,
		ReminderInterval:      4 * time.Hour,
		TriggerThreshold:      25.0, // 25% increase
		ConsecutiveOKRequired: 2,
	})

	alertManager.RegisterPolicy("health_aggregate", "avg_hf_drop", alerts.AlertPolicy{
		MinValueChange:        0.02, // 0.02 HF change
		CooldownWarning:       30 * time.Minute,
		CooldownCritical:      15 * time.Minute,
		ReminderInterval:      2 * time.Hour,
		TriggerThreshold:      0.1, // 0.1 HF drop
		ConsecutiveOKRequired: 2,
	})

	alertManager.RegisterPolicy("health_aggregate", "withdrawal_spike", alerts.AlertPolicy{
		MinValueChange:        2.0, // 2% change
		CooldownWarning:       1 * time.Hour,
		CooldownCritical:      30 * time.Minute,
		ReminderInterval:      0,
		TriggerThreshold:      10.0, // 10% decrease
		ConsecutiveOKRequired: 2,
	})

	alertManager.RegisterPolicy("health_aggregate", "borrow_spike", alerts.AlertPolicy{
		MinValueChange:        2.0, // 2% change
		CooldownWarning:       1 * time.Hour,
		CooldownCritical:      30 * time.Minute,
		ReminderInterval:      0,
		TriggerThreshold:      10.0, // 10% increase
		ConsecutiveOKRequired: 2,
	})

	now := time.Now()
	return &HealthAggregateJob{
		db:                  db,
		alertManager:        alertManager,
		lastRiskyCountCheck: now,
		last24hCheckTime:    now.Add(-24 * time.Hour),
		last24hSupplyTime:   now.Add(-24 * time.Hour),
		last24hBorrowTime:   now.Add(-24 * time.Hour),
	}, nil
}

func (j *HealthAggregateJob) Name() string {
	return "health_aggregate"
}

func (j *HealthAggregateJob) Interval() time.Duration {
	return 5 * time.Minute
}

func (j *HealthAggregateJob) Run(ctx context.Context) error {
	metrics, err := j.getAggregateMetrics(ctx)
	if err != nil {
		return fmt.Errorf("failed to get aggregate metrics: %w", err)
	}

	// Check 1: Risky position count spike (>25% increase in 24hrs)
	j.checkRiskyCountSpike(ctx, metrics)

	// Check 2: Average HF drop (>0.1 drop within 1hr)
	j.checkAvgHealthFactorDrop(ctx, metrics)

	// Check 3: Withdrawal spike (>10% decrease in supply over 24hrs)
	j.checkWithdrawalSpike(ctx, metrics)

	// Check 4: Borrow spike (>10% increase in borrows over 24hrs)
	j.checkBorrowSpike(ctx, metrics)

	log.Printf("[%s] risky positions: %d/%d, weighted avg HF: %.4f, supply: $%s, borrow: $%s",
		j.Name(), metrics.RiskyPositions, metrics.TotalPositions, metrics.WeightedAvgHF,
		formatUSD(metrics.TotalCollateralUSD), formatUSD(metrics.TotalBorrowUSD))

	return nil
}

func (j *HealthAggregateJob) getAggregateMetrics(ctx context.Context) (*aggregateMetrics, error) {
	query := `
		SELECT 
			COUNT(*) as total_positions,
			COUNT(*) FILTER (WHERE health_factor > 0 AND health_factor < 1.2) as risky_positions,
			COALESCE(SUM(total_supplied), 0) as total_collateral,
			COALESCE(SUM(total_borrowed), 0) as total_borrow,
			COALESCE(SUM(LEAST(health_factor, 100) * total_borrowed), 0) as weighted_hf_sum
		FROM public."UserPositions"
		WHERE health_factor > 0 AND health_factor < 1000
	`

	var metrics aggregateMetrics
	var totalCollateral, totalBorrow, weightedHFSum sql.NullFloat64

	err := j.db.QueryRowContext(ctx, query).Scan(
		&metrics.TotalPositions,
		&metrics.RiskyPositions,
		&totalCollateral,
		&totalBorrow,
		&weightedHFSum,
	)
	if err != nil {
		return nil, err
	}

	metrics.TotalCollateralUSD = totalCollateral.Float64
	metrics.TotalBorrowUSD = totalBorrow.Float64

	// Calculate weighted average health factor (borrow-weighted)
	// Weighted Avg HF = Σ(HF_i × Borrow_i) / Total_Borrow
	// This weights users with more debt more heavily, which is appropriate for risk assessment
	if metrics.TotalBorrowUSD > 0 && weightedHFSum.Valid {
		metrics.WeightedAvgHF = weightedHFSum.Float64 / metrics.TotalBorrowUSD
		// Cap the weighted average HF to a reasonable value
		if metrics.WeightedAvgHF > 100 {
			metrics.WeightedAvgHF = 100.0
		}
	} else {
		metrics.WeightedAvgHF = 999.0 // No borrows = use large value (no risk)
	}

	return &metrics, nil
}

func (j *HealthAggregateJob) checkRiskyCountSpike(ctx context.Context, metrics *aggregateMetrics) {
	now := time.Now()

	// Check if 24 hours have passed since we stored the baseline
	if now.Sub(j.last24hCheckTime) >= 24*time.Hour {
		// Calculate percentage increase
		var percentIncrease float64
		if j.last24hRiskyCount > 0 {
			percentIncrease = float64(metrics.RiskyPositions-j.last24hRiskyCount) / float64(j.last24hRiskyCount) * 100
		} else if metrics.RiskyPositions > 0 {
			percentIncrease = 100.0 // 0 to any number is 100% increase
		}

		key := alerts.AlertKey{
			Job:    j.Name(),
			Entity: "protocol",
			Metric: "risky_count_spike",
		}

		var severity alerts.Severity
		switch {
		case percentIncrease >= 50:
			severity = alerts.SeverityCritical
		case percentIncrease >= 25:
			severity = alerts.SeverityWarning
		default:
			severity = alerts.SeverityOK
		}

		summary := ""
		details := fmt.Sprintf(
			"Risky positions (HF < 1.2): %d (24h ago: %d)\nChange: %.1f%%\nTotal positions: %d",
			metrics.RiskyPositions,
			j.last24hRiskyCount,
			percentIncrease,
			metrics.TotalPositions,
		)

		if err := j.alertManager.Observe(ctx, key, severity, percentIncrease, summary, details, true, ""); err != nil {
			log.Printf("[%s] failed to observe risky count spike: %v", j.Name(), err)
		}

		// Update baseline for next 24h check
		j.last24hRiskyCount = metrics.RiskyPositions
		j.last24hCheckTime = now
	}
}

func (j *HealthAggregateJob) checkAvgHealthFactorDrop(ctx context.Context, metrics *aggregateMetrics) {
	now := time.Now()
	timeSinceLastCheck := now.Sub(j.lastRiskyCountCheck)

	// Only check if at least 1 hour has passed and we have a previous value
	if timeSinceLastCheck >= 1*time.Hour && j.lastAvgHealthFactor > 0 {
		hfDrop := j.lastAvgHealthFactor - metrics.WeightedAvgHF

		key := alerts.AlertKey{
			Job:    j.Name(),
			Entity: "protocol",
			Metric: "avg_hf_drop",
		}

		var severity alerts.Severity
		switch {
		case hfDrop >= 0.2:
			severity = alerts.SeverityCritical
		case hfDrop >= 0.1:
			severity = alerts.SeverityWarning
		case hfDrop >= 0.05:
			severity = alerts.SeverityWarning
		default:
			severity = alerts.SeverityOK
		}

		summary := ""
		details := fmt.Sprintf(
			"Weighted Avg HF: %.4f (1h ago: %.4f)\nDrop: %.4f\nTotal Collateral: $%s\nTotal Borrow: $%s",
			metrics.WeightedAvgHF,
			j.lastAvgHealthFactor,
			hfDrop,
			formatUSD(metrics.TotalCollateralUSD),
			formatUSD(metrics.TotalBorrowUSD),
		)

		if err := j.alertManager.Observe(ctx, key, severity, hfDrop, summary, details, true, ""); err != nil {
			log.Printf("[%s] failed to observe avg HF drop: %v", j.Name(), err)
		}

		// Update for next check
		j.lastRiskyCountCheck = now
	}

	// Always update last value
	j.lastAvgHealthFactor = metrics.WeightedAvgHF
}

func (j *HealthAggregateJob) checkWithdrawalSpike(ctx context.Context, metrics *aggregateMetrics) {
	now := time.Now()

	// Check if 24 hours have passed since baseline
	if now.Sub(j.last24hSupplyTime) >= 24*time.Hour && j.last24hTotalSupply > 0 {
		// Calculate percentage decrease
		change := metrics.TotalCollateralUSD - j.last24hTotalSupply
		percentChange := (change / j.last24hTotalSupply) * 100

		// Negative change = withdrawal (supply decrease)
		percentDecrease := -percentChange

		key := alerts.AlertKey{
			Job:    j.Name(),
			Entity: "protocol",
			Metric: "withdrawal_spike",
		}

		var severity alerts.Severity
		switch {
		case percentDecrease >= 20:
			severity = alerts.SeverityCritical
		case percentDecrease >= 10:
			severity = alerts.SeverityWarning
		default:
			severity = alerts.SeverityOK
		}

		summary := ""
		details := fmt.Sprintf(
			"Supply Change: %.2f%% (24h)\nCurrent Supply: $%s\n24h ago: $%s\nChange: $%s",
			percentChange,
			formatUSD(metrics.TotalCollateralUSD),
			formatUSD(j.last24hTotalSupply),
			formatUSD(change),
		)

		if err := j.alertManager.Observe(ctx, key, severity, percentDecrease, summary, details, true, ""); err != nil {
			log.Printf("[%s] failed to observe withdrawal spike: %v", j.Name(), err)
		}

		// Update baseline
		j.last24hTotalSupply = metrics.TotalCollateralUSD
		j.last24hSupplyTime = now
	} else if j.last24hTotalSupply == 0 {
		// Initialize baseline
		j.last24hTotalSupply = metrics.TotalCollateralUSD
		j.last24hSupplyTime = now
	}
}

func (j *HealthAggregateJob) checkBorrowSpike(ctx context.Context, metrics *aggregateMetrics) {
	now := time.Now()

	// Check if 24 hours have passed since baseline
	if now.Sub(j.last24hBorrowTime) >= 24*time.Hour && j.last24hTotalBorrow > 0 {
		// Calculate percentage increase
		change := metrics.TotalBorrowUSD - j.last24hTotalBorrow
		percentChange := (change / j.last24hTotalBorrow) * 100

		key := alerts.AlertKey{
			Job:    j.Name(),
			Entity: "protocol",
			Metric: "borrow_spike",
		}

		var severity alerts.Severity
		switch {
		case percentChange >= 20:
			severity = alerts.SeverityCritical
		case percentChange >= 10:
			severity = alerts.SeverityWarning
		default:
			severity = alerts.SeverityOK
		}

		summary := ""
		details := fmt.Sprintf(
			"Borrow Change: %.2f%% (24h)\nCurrent Borrow: $%s\n24h ago: $%s\nChange: $%s",
			percentChange,
			formatUSD(metrics.TotalBorrowUSD),
			formatUSD(j.last24hTotalBorrow),
			formatUSD(change),
		)

		if err := j.alertManager.Observe(ctx, key, severity, percentChange, summary, details, true, ""); err != nil {
			log.Printf("[%s] failed to observe borrow spike: %v", j.Name(), err)
		}

		// Update baseline
		j.last24hTotalBorrow = metrics.TotalBorrowUSD
		j.last24hBorrowTime = now
	} else if j.last24hTotalBorrow == 0 {
		// Initialize baseline
		j.last24hTotalBorrow = metrics.TotalBorrowUSD
		j.last24hBorrowTime = now
	}
}

func (j *HealthAggregateJob) Close() error {
	if j.db != nil {
		return j.db.Close()
	}
	return nil
}
