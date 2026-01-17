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

const (
	healthFactorThreshold = 1.5
	queryLimit            = 100
)

type userPosition struct {
	Address      string
	HealthFactor float64
	TotalSupply  float64
	TotalBorrow  float64
}

// HealthJobV2 implements health factor monitoring with stateful alerting
type HealthJobV2 struct {
	db            *sql.DB
	alertManager  *alerts.Manager
	lastDataCheck time.Time
}

// NewHealthJobV2 creates a new health factor monitoring job
func NewHealthJobV2(databaseURL string, alertManager *alerts.Manager) (*HealthJobV2, error) {
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

	// Register policies for health factor alerts
	// No reminders for business alerts - only new incidents, escalations, and critical updates
	alertManager.RegisterPolicy("health_factor", "position_risk", alerts.AlertPolicy{
		MinValueChange:        0.05, // HF change of 0.05
		CooldownWarning:       30 * time.Minute,
		CooldownCritical:      10 * time.Minute,
		ReminderInterval:      0,   // No reminders (handled at manager level)
		TriggerThreshold:      1.5, // Warning at HF < 1.5
		ConsecutiveOKRequired: 2,
	})

	alertManager.RegisterPolicy("health_factor", "data_staleness", alerts.AlertPolicy{
		MinValueChange:        60.0, // 60 minutes change
		CooldownWarning:       1 * time.Hour,
		CooldownCritical:      30 * time.Minute,
		ReminderInterval:      4 * time.Hour,
		ConsecutiveOKRequired: 1,
	})

	return &HealthJobV2{
		db:            db,
		alertManager:  alertManager,
		lastDataCheck: time.Now(),
	}, nil
}

func (j *HealthJobV2) Name() string {
	return "health_factor"
}

func (j *HealthJobV2) Interval() time.Duration {
	return 5 * time.Minute
}

func (j *HealthJobV2) Run(ctx context.Context) error {
	// Check data freshness
	if err := j.checkDataFreshness(ctx); err != nil {
		j.observeDatabaseError(ctx, "freshness_check", err)
		return fmt.Errorf("failed to check data freshness: %w", err)
	}

	// Get risky positions
	positions, err := j.getRiskyPositions(ctx)
	if err != nil {
		j.observeDatabaseError(ctx, "query_positions", err)
		return fmt.Errorf("failed to get risky positions: %w", err)
	}

	// Clear database error if we got here successfully
	j.clearDatabaseError(ctx)

	// Process each position
	for _, pos := range positions {
		// Log every position (for debugging/monitoring)
		// Individual position alerts disabled - aggregate health monitoring handles systemic risk
		log.Printf("[%s] %s: HF=%.4f, supply=$%s, borrow=$%s",
			j.Name(), pos.Address, pos.HealthFactor,
			formatUSD(pos.TotalSupply), formatUSD(pos.TotalBorrow))
	}

	log.Printf("[%s] processed %d risky positions", j.Name(), len(positions))
	return nil
}

func (j *HealthJobV2) checkDataFreshness(ctx context.Context) error {
	var lastUpdate time.Time
	query := `SELECT MAX(last_updated) FROM public."UserPositions"`

	err := j.db.QueryRowContext(ctx, query).Scan(&lastUpdate)
	if err != nil {
		return err
	}

	timeSinceUpdate := time.Since(lastUpdate)

	key := alerts.AlertKey{
		Job:    j.Name(),
		Entity: "database",
		Metric: "data_staleness",
	}

	var severity alerts.Severity
	switch {
	case timeSinceUpdate > 10*time.Hour:
		severity = alerts.SeverityCritical
	case timeSinceUpdate > 5*time.Hour:
		severity = alerts.SeverityWarning
	default:
		severity = alerts.SeverityOK
	}

	summary := "UserPositions data freshness"
	details := fmt.Sprintf(
		"Last update: %s\nAge: %.1f hours",
		lastUpdate.Format("2006-01-02 15:04:05 UTC"),
		timeSinceUpdate.Hours(),
	)

	if err := j.alertManager.Observe(ctx, key, severity, timeSinceUpdate.Hours(), summary, details, false, ""); err != nil {
		log.Printf("[%s] failed to observe data freshness: %v", j.Name(), err)
	}

	return nil
}

func (j *HealthJobV2) getRiskyPositions(ctx context.Context) ([]userPosition, error) {
	query := `
		SELECT 
			user_address,
			health_factor,
			total_supplied,
			total_borrowed
		FROM public."UserPositions"
		WHERE health_factor > 0 
			AND health_factor < $1
		ORDER BY health_factor ASC
		LIMIT $2
	`

	rows, err := j.db.QueryContext(ctx, query, healthFactorThreshold, queryLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var positions []userPosition
	for rows.Next() {
		var pos userPosition
		err := rows.Scan(
			&pos.Address,
			&pos.HealthFactor,
			&pos.TotalSupply,
			&pos.TotalBorrow,
		)
		if err != nil {
			log.Printf("[%s] scan error: %v", j.Name(), err)
			continue
		}
		positions = append(positions, pos)
	}

	return positions, rows.Err()
}

func (j *HealthJobV2) observeDatabaseError(ctx context.Context, operation string, err error) {
	key := alerts.AlertKey{
		Job:    j.Name(),
		Entity: "database",
		Metric: operation + "_error",
	}

	summary := fmt.Sprintf("Database operation failed: %s", operation)
	details := fmt.Sprintf("Error: %v", err)

	j.alertManager.Observe(ctx, key, alerts.SeverityCritical, 1.0, summary, details, false, "")
}

func (j *HealthJobV2) clearDatabaseError(ctx context.Context) {
	// Clear any database errors
	for _, operation := range []string{"freshness_check", "query_positions"} {
		key := alerts.AlertKey{
			Job:    j.Name(),
			Entity: "database",
			Metric: operation + "_error",
		}
		j.alertManager.Observe(ctx, key, alerts.SeverityOK, 0, "Database operational", "", false, "")
	}
}

func (j *HealthJobV2) Close() error {
	if j.db != nil {
		return j.db.Close()
	}
	return nil
}

func formatUSD(value float64) string {
	if value >= 1_000_000 {
		return fmt.Sprintf("%.2fM", value/1_000_000)
	} else if value >= 1_000 {
		return fmt.Sprintf("%.2fK", value/1_000)
	}
	return fmt.Sprintf("%.2f", value)
}
