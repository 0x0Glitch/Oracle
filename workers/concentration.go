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

// ConcentrationJob monitors whale positions and borrow concentration
type ConcentrationJob struct {
	db             *sql.DB
	alertManager   *alerts.Manager
	previousWhales map[string]bool // Track whale addresses from previous run
}

type whalePosition struct {
	Address       string
	TotalSupplied float64
	Percentage    float64
}

// NewConcentrationJob creates a new concentration risk monitoring job
func NewConcentrationJob(databaseURL string, alertManager *alerts.Manager) (*ConcentrationJob, error) {
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

	// Register policies for concentration alerts
	alertManager.RegisterPolicy("concentration", "whale_supply", alerts.AlertPolicy{
		MinValueChange:        1.0, // 1% change in concentration
		CooldownWarning:       1 * time.Hour,
		CooldownCritical:      30 * time.Minute,
		ReminderInterval:      0,
		TriggerThreshold:      10.0, // 10% of total supply
		ConsecutiveOKRequired: 2,
	})

	alertManager.RegisterPolicy("concentration", "borrow_top10", alerts.AlertPolicy{
		MinValueChange:        2.0, // 2% change
		CooldownWarning:       1 * time.Hour,
		CooldownCritical:      30 * time.Minute,
		ReminderInterval:      0,
		TriggerThreshold:      80.0, // 80% concentration
		ConsecutiveOKRequired: 2,
	})

	alertManager.RegisterPolicy("concentration", "borrow_single", alerts.AlertPolicy{
		MinValueChange:        2.0, // 2% change
		CooldownWarning:       1 * time.Hour,
		CooldownCritical:      30 * time.Minute,
		ReminderInterval:      0,
		TriggerThreshold:      40.0, // 40% concentration
		ConsecutiveOKRequired: 2,
	})

	return &ConcentrationJob{
		db:             db,
		alertManager:   alertManager,
		previousWhales: make(map[string]bool),
	}, nil
}

func (j *ConcentrationJob) Name() string {
	return "concentration"
}

func (j *ConcentrationJob) Interval() time.Duration {
	return 10 * time.Minute
}

func (j *ConcentrationJob) Run(ctx context.Context) error {
	// Check whale positions (>10% of supply)
	if err := j.checkWhalePositions(ctx); err != nil {
		log.Printf("[%s] whale check failed: %v", j.Name(), err)
	}

	// Check borrow concentration
	if err := j.checkBorrowConcentration(ctx); err != nil {
		log.Printf("[%s] borrow concentration check failed: %v", j.Name(), err)
	}

	return nil
}

func (j *ConcentrationJob) checkWhalePositions(ctx context.Context) error {
	query := `
		WITH total AS (
			SELECT SUM(total_supplied) as total_supply
			FROM public."UserPositions"
			WHERE total_supplied > 0
		)
		SELECT
			user_address,
			total_supplied,
			(total_supplied / total.total_supply * 100) as percentage
		FROM public."UserPositions", total
		WHERE total_supplied > 0
			AND (total_supplied / total.total_supply * 100) >= 10
		ORDER BY percentage DESC
	`

	rows, err := j.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("whale query failed: %w", err)
	}
	defer rows.Close()

	currentWhales := make(map[string]bool)
	whaleCount := 0
	for rows.Next() {
		var whale whalePosition
		if err := rows.Scan(&whale.Address, &whale.TotalSupplied, &whale.Percentage); err != nil {
			log.Printf("[%s] scan error: %v", j.Name(), err)
			continue
		}

		whaleCount++
		currentWhales[whale.Address] = true

		// Alert for each whale position
		key := alerts.AlertKey{
			Job:    j.Name(),
			Entity: whale.Address,
			Metric: "whale_supply",
		}

		var severity alerts.Severity
		switch {
		case whale.Percentage >= 20:
			severity = alerts.SeverityCritical
		case whale.Percentage >= 10:
			severity = alerts.SeverityWarning
		default:
			severity = alerts.SeverityOK
		}

		// Log whale position
		log.Printf("[%s] whale %s: concentration=%.2f%%, supply=$%s, severity=%s",
			j.Name(), whale.Address, whale.Percentage, formatUSD(whale.TotalSupplied), severity)

		summary := ""
		details := fmt.Sprintf(
			"Supply Concentration: %.2f%%\nSupply: $%s\nAddress: %s",
			whale.Percentage,
			formatUSD(whale.TotalSupplied),
			whale.Address,
		)

		if err := j.alertManager.Observe(ctx, key, severity, whale.Percentage, summary, details, true, ""); err != nil {
			log.Printf("[%s] failed to observe whale alert: %v", j.Name(), err)
		}
	}

	// Clear alerts for whales that dropped below threshold
	for addr := range j.previousWhales {
		if !currentWhales[addr] {
			key := alerts.AlertKey{
				Job:    j.Name(),
				Entity: addr,
				Metric: "whale_supply",
			}
			j.alertManager.Observe(ctx, key, alerts.SeverityOK, 0, "", "", false, "")
		}
	}

	// Update previous whales for next iteration
	j.previousWhales = currentWhales

	if whaleCount > 0 {
		log.Printf("[%s] found %d whale positions (>10%% supply)", j.Name(), whaleCount)
	}

	return rows.Err()
}

func (j *ConcentrationJob) checkBorrowConcentration(ctx context.Context) error {
	// Get total borrows
	var totalBorrows float64
	err := j.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(total_borrowed), 0)
		FROM public."UserPositions"
		WHERE total_borrowed > 0
	`).Scan(&totalBorrows)
	if err != nil {
		return fmt.Errorf("total borrows query failed: %w", err)
	}

	if totalBorrows == 0 {
		// Clear any existing borrow concentration alerts when there are no borrows
		j.alertManager.Observe(ctx, alerts.AlertKey{Job: j.Name(), Entity: "protocol", Metric: "borrow_top10"}, alerts.SeverityOK, 0, "", "", false, "")
		return nil // No borrows to check
	}

	// Get top 10 borrowers
	query := `
		SELECT 
			user_address,
			total_borrowed,
			(total_borrowed / $1 * 100) as percentage
		FROM public."UserPositions"
		WHERE total_borrowed > 0
		ORDER BY total_borrowed DESC
		LIMIT 10
	`

	rows, err := j.db.QueryContext(ctx, query, totalBorrows)
	if err != nil {
		return fmt.Errorf("top borrowers query failed: %w", err)
	}
	defer rows.Close()

	var top10Sum float64
	var maxSingle float64
	var maxAddress string

	for rows.Next() {
		var addr string
		var borrowed, percentage float64
		if err := rows.Scan(&addr, &borrowed, &percentage); err != nil {
			log.Printf("[%s] scan error: %v", j.Name(), err)
			continue
		}

		top10Sum += borrowed
		if borrowed > maxSingle {
			maxSingle = borrowed
			maxAddress = addr
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Calculate percentages
	top10Percentage := (top10Sum / totalBorrows) * 100
	maxSinglePercentage := (maxSingle / totalBorrows) * 100

	// Log borrow concentration metrics
	log.Printf("[%s] borrow concentration: top10=%.2f%%, single_max=%.2f%%, total=$%s",
		j.Name(), top10Percentage, maxSinglePercentage, formatUSD(totalBorrows))

	// Alert for top 10 concentration
	{
		key := alerts.AlertKey{
			Job:    j.Name(),
			Entity: "protocol",
			Metric: "borrow_top10",
		}

		var severity alerts.Severity
		switch {
		case top10Percentage >= 90:
			severity = alerts.SeverityCritical
		case top10Percentage >= 80:
			severity = alerts.SeverityWarning
		default:
			severity = alerts.SeverityOK
		}

		summary := ""
		details := fmt.Sprintf(
			"Top 10 Borrow Concentration: %.2f%%\nTop 10 Borrows: $%s\nTotal Borrows: $%s",
			top10Percentage,
			formatUSD(top10Sum),
			formatUSD(totalBorrows),
		)

		if err := j.alertManager.Observe(ctx, key, severity, top10Percentage, summary, details, true, ""); err != nil {
			log.Printf("[%s] failed to observe top10 alert: %v", j.Name(), err)
		}
	}

	// Alert for single wallet concentration
	{
		key := alerts.AlertKey{
			Job:    j.Name(),
			Entity: maxAddress,
			Metric: "borrow_single",
		}

		var severity alerts.Severity
		switch {
		case maxSinglePercentage >= 50:
			severity = alerts.SeverityCritical
		case maxSinglePercentage >= 40:
			severity = alerts.SeverityWarning
		default:
			severity = alerts.SeverityOK
		}

		summary := ""
		details := fmt.Sprintf(
			"Single Wallet Borrow: %.2f%%\nBorrow: $%s\nTotal Borrows: $%s\nAddress: %s",
			maxSinglePercentage,
			formatUSD(maxSingle),
			formatUSD(totalBorrows),
			maxAddress,
		)

		if err := j.alertManager.Observe(ctx, key, severity, maxSinglePercentage, summary, details, true, ""); err != nil {
			log.Printf("[%s] failed to observe single wallet alert: %v", j.Name(), err)
		}
	}

	log.Printf("[%s] top10: %.1f%%, max single: %.1f%%", j.Name(), top10Percentage, maxSinglePercentage)
	return nil
}

func (j *ConcentrationJob) Close() error {
	if j.db != nil {
		return j.db.Close()
	}
	return nil
}
