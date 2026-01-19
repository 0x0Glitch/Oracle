package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/0x0Glitch/alerts"
	"github.com/0x0Glitch/config"
)

const (
	maxConcurrentTokens = 5
	httpTimeout         = 10 * time.Second
	maxRetries          = 3
	retryDelay          = 500 * time.Millisecond
)

// OracleMonitor monitors oracle prices for a specific chain
type OracleMonitor struct {
	chain          ChainConfig
	client         *ethclient.Client
	oracle         *OracleCaller
	alchemyKey     string
	alertManager   *alerts.Manager
	httpClient     *http.Client
	config         *config.OracleConfig
	mu             sync.Mutex
	lastSuccess    time.Time
	consecutiveErr int
	failures       int
}

type tokenResult struct {
	symbol       string
	onchainPrice float64
	dexPrice     float64
	deviation    float64
	err          error
}

// NewOracleMonitor creates a new oracle monitor for a specific chain
func NewOracleMonitor(
	chain ChainConfig,
	client *ethclient.Client,
	alchemyKey string,
	alertManager *alerts.Manager,
	cfg *config.OracleConfig,
) (*OracleMonitor, error) {
	oracle, err := NewOracleCaller(common.HexToAddress(chain.OracleAddress), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create oracle caller: %w", err)
	}

	// Register alert policies
	registerOraclePolicies(alertManager, cfg, string(chain.ID))

	return &OracleMonitor{
		chain:        chain,
		client:       client,
		oracle:       oracle,
		alchemyKey:   alchemyKey,
		alertManager: alertManager,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
		config:      cfg,
		lastSuccess: time.Now(),
	}, nil
}

func (m *OracleMonitor) Name() string {
	return fmt.Sprintf("oracle_%s", m.chain.ID)
}

func (m *OracleMonitor) Interval() time.Duration {
	if m.config != nil && m.config.CheckIntervalSeconds > 0 {
		return time.Duration(m.config.CheckIntervalSeconds) * time.Second
	}
	return 30 * time.Second
}

func (m *OracleMonitor) Run(ctx context.Context) error {
	log.Printf("[%s][%s] checking %d tokens", m.Name(), m.chain.Name, len(m.chain.Tokens))

	// Simple circuit breaker - skip if too many recent failures
	m.mu.Lock()
	currentFailures := m.failures
	m.mu.Unlock()

	if currentFailures >= 5 {
		log.Printf("[%s][%s] circuit open (%d failures), skipping check", m.Name(), m.chain.Name, currentFailures)
		return errors.New("circuit breaker open")
	}

	results := m.checkAllTokens(ctx)

	var errorResults []tokenResult
	successCount := 0

	for _, result := range results {
		if result.err != nil {
			errorResults = append(errorResults, result)
			log.Printf("[%s][%s] %s: %v", m.Name(), m.chain.Name, result.symbol, result.err)
			m.observeTokenError(ctx, result.symbol, result.err)
			continue
		}

		successCount++
		m.processTokenResult(ctx, result)
	}

	// Update health
	m.updateSystemHealth(ctx, successCount, errorResults)

	// Update circuit breaker
	tokenCount := len(m.chain.Tokens)
	if tokenCount == 0 {
		return nil // No tokens to check
	}
	errorRate := float64(len(errorResults)) / float64(tokenCount)
	m.mu.Lock()
	if errorRate > 0.5 {
		m.failures++
	} else {
		m.failures = 0
	}
	m.mu.Unlock()

	if errorRate > 0.5 {
		return fmt.Errorf("high error rate: %.1f%%", errorRate*100)
	}

	return nil
}

func (m *OracleMonitor) checkAllTokens(ctx context.Context) []tokenResult {
	sem := make(chan struct{}, maxConcurrentTokens)
	resultChan := make(chan tokenResult, len(m.chain.Tokens))
	var wg sync.WaitGroup

	for symbol, meta := range m.chain.Tokens {
		wg.Add(1)
		go func(sym string, token TokenMeta) {
			sem <- struct{}{} // Acquire semaphore first
			defer func() {
				<-sem // Release semaphore in defer
				if r := recover(); r != nil {
					log.Printf("[%s][%s] panic checking %s: %v", m.Name(), m.chain.Name, sym, r)
					resultChan <- tokenResult{symbol: sym, err: fmt.Errorf("panic: %v", r)}
				}
				wg.Done()
			}()

			result := m.checkToken(ctx, sym, token)
			resultChan <- result
		}(symbol, meta)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var results []tokenResult
	for result := range resultChan {
		results = append(results, result)
	}
	return results
}

func (m *OracleMonitor) checkToken(ctx context.Context, symbol string, meta TokenMeta) tokenResult {
	result := tokenResult{symbol: symbol}

	if meta.Decimals > 36 {
		result.err = fmt.Errorf("invalid decimals: %d", meta.Decimals)
		return result
	}

	// Get onchain price with retry
	var onchainPrice float64
	for attempt := 0; attempt < maxRetries; attempt++ {
		price, err := m.getOnchainPrice(ctx, meta.MTokAddr, meta.Decimals)
		if err == nil {
			onchainPrice = price
			break
		}
		if attempt == maxRetries-1 {
			result.err = fmt.Errorf("onchain price: %w", err)
			return result
		}
		time.Sleep(retryDelay * time.Duration(attempt+1))
	}
	result.onchainPrice = onchainPrice

	// Get DEX price with retry (skip for tokens without DEX price source)
	var dexPrice float64
	if !meta.SkipDEXPrice {
		for attempt := 0; attempt < maxRetries; attempt++ {
			price, err := m.getAlchemyPrice(ctx, meta)
			if err == nil {
				dexPrice = price
				break
			}
			if attempt == maxRetries-1 {
				result.err = fmt.Errorf("dex price: %w", err)
				return result
			}
			time.Sleep(retryDelay * time.Duration(attempt+1))
		}
		result.dexPrice = dexPrice
	}

	// Calculate deviation
	if meta.IsStablecoin && meta.PegValue > 0 {
		result.deviation = math.Abs((onchainPrice-meta.PegValue)/meta.PegValue) * 100
	} else if dexPrice > 0 {
		result.deviation = math.Abs((onchainPrice-dexPrice)/dexPrice) * 100
	} else if meta.SkipDEXPrice {
		// Native tokens without DEX price - only log oracle price, no deviation check
		result.deviation = 0
	} else {
		// Cannot calculate deviation without a reference price
		result.err = fmt.Errorf("cannot calculate deviation: no reference price (dex=%.6f, peg=%.2f)", dexPrice, meta.PegValue)
		return result
	}

	return result
}

func (m *OracleMonitor) processTokenResult(ctx context.Context, result tokenResult) {
	meta, exists := m.chain.Tokens[result.symbol]
	if !exists {
		log.Printf("[%s][%s] token %s not found in config", m.Name(), m.chain.Name, result.symbol)
		return
	}
	severity := m.classifyDeviation(result.deviation, meta)

	if meta.IsStablecoin {
		log.Printf("[%s][%s] %s: dev=%.4f%%, onchain=$%.6f, peg=$%.2f, dex=$%.6f, sev=%s",
			m.Name(), m.chain.Name, result.symbol, result.deviation, result.onchainPrice, meta.PegValue, result.dexPrice, severity)
	} else {
		log.Printf("[%s][%s] %s: dev=%.4f%%, onchain=$%.6f, dex=$%.6f, sev=%s",
			m.Name(), m.chain.Name, result.symbol, result.deviation, result.onchainPrice, result.dexPrice, severity)
	}

	key := alerts.AlertKey{
		Job:    m.Name(),
		Entity: meta.TableName,
		Metric: m.getMetricName(meta),
	}

	details := m.formatAlertDetails(result, meta)
	slackMsg := m.formatSlackAlert(result, meta, severity)

	m.alertManager.Observe(ctx, key, severity, result.deviation, "", details, true, slackMsg)
}

func (m *OracleMonitor) formatAlertDetails(result tokenResult, meta TokenMeta) string {
	if meta.IsStablecoin {
		return fmt.Sprintf("Token: %s\nChain: %s\nDeviation: %.2f%%\nOnchain: $%.6f\nPeg: $%.2f\nDEX: $%.6f",
			meta.TableName, m.chain.Name, result.deviation, result.onchainPrice, meta.PegValue, result.dexPrice)
	}
	return fmt.Sprintf("Token: %s\nChain: %s\nDeviation: %.2f%%\nOnchain: $%.6f\nDEX: $%.6f",
		meta.TableName, m.chain.Name, result.deviation, result.onchainPrice, result.dexPrice)
}

func (m *OracleMonitor) formatSlackAlert(result tokenResult, meta TokenMeta, severity alerts.Severity) string {
	if meta.IsStablecoin {
		return fmt.Sprintf("ALERT: STABLECOIN DEPEG\nToken: %s\nChain: %s\nDeviation: %.2f%%\nOnchain: $%.6f\nDEX: $%.6f",
			meta.TableName, m.chain.Name, result.deviation, result.onchainPrice, result.dexPrice)
	}
	return fmt.Sprintf("ALERT: ORACLE PRICE DEVIATION\nToken: %s\nChain: %s\nDeviation: %.2f%%\nOnchain: $%.6f\nDEX: $%.6f",
		meta.TableName, m.chain.Name, result.deviation, result.onchainPrice, result.dexPrice)
}

func (m *OracleMonitor) getOnchainPrice(ctx context.Context, mTokenAddr string, decimals int) (float64, error) {
	addr := common.HexToAddress(mTokenAddr)
	price, err := m.oracle.GetUnderlyingPrice(&bind.CallOpts{Context: ctx}, addr)
	if err != nil {
		return 0, err
	}

	priceFloat := new(big.Float).SetInt(price)
	exponent := 36 - decimals
	divisor := new(big.Float).SetFloat64(math.Pow(10, float64(exponent)))
	priceFloat.Quo(priceFloat, divisor)

	result, _ := priceFloat.Float64()
	return result, nil
}

func (m *OracleMonitor) getAlchemyPrice(ctx context.Context, meta TokenMeta) (float64, error) {
	if meta.PriceAddress == "" {
		return 0, fmt.Errorf("no price address")
	}

	url := fmt.Sprintf("https://api.g.alchemy.com/prices/v1/%s/tokens/by-address", m.alchemyKey)
	payload := map[string]interface{}{
		"addresses": []map[string]string{
			{"network": m.chain.PriceNetwork, "address": meta.PriceAddress},
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("API status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			Prices []struct {
				Currency string `json:"currency"`
				Value    string `json:"value"`
			} `json:"prices"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if len(result.Data) == 0 || len(result.Data[0].Prices) == 0 {
		return 0, fmt.Errorf("no price data")
	}

	for _, p := range result.Data[0].Prices {
		if p.Currency == "usd" {
			return strconv.ParseFloat(p.Value, 64)
		}
	}

	return 0, fmt.Errorf("no USD price")
}

func (m *OracleMonitor) classifyDeviation(deviation float64, meta TokenMeta) alerts.Severity {
	if m.config == nil {
		return alerts.SeverityOK
	}

	if meta.IsStablecoin {
		if deviation >= m.config.Stablecoin.CriticalThresholdPercent {
			return alerts.SeverityCritical
		}
		if deviation >= m.config.Stablecoin.WarningThresholdPercent {
			return alerts.SeverityWarning
		}
		return alerts.SeverityOK
	}

	if deviation >= m.config.Volatile.CriticalThresholdPercent {
		return alerts.SeverityCritical
	}
	if deviation >= m.config.Volatile.WarningThresholdPercent {
		return alerts.SeverityWarning
	}
	return alerts.SeverityOK
}

func (m *OracleMonitor) getMetricName(meta TokenMeta) string {
	if meta.IsStablecoin {
		return "price_deviation_stable"
	}
	return "price_deviation_volatile"
}

func (m *OracleMonitor) observeTokenError(ctx context.Context, symbol string, err error) {
	key := alerts.AlertKey{Job: m.Name(), Entity: symbol, Metric: "token_error"}
	details := fmt.Sprintf("Chain: %s\nToken: %s\nError: %v", m.chain.Name, symbol, err)
	m.alertManager.Observe(ctx, key, alerts.SeverityWarning, 1.0, "", details, false, "")
}

func (m *OracleMonitor) updateSystemHealth(ctx context.Context, successCount int, errors []tokenResult) {
	m.mu.Lock()
	if successCount > 0 {
		m.lastSuccess = time.Now()
		m.consecutiveErr = 0
	} else {
		m.consecutiveErr++
	}
	lastSuccess := m.lastSuccess
	consecutiveErr := m.consecutiveErr
	m.mu.Unlock()

	tokenCount := len(m.chain.Tokens)
	if tokenCount == 0 {
		return // No tokens to report on
	}
	errorRate := float64(len(errors)) / float64(tokenCount) * 100

	var severity alerts.Severity
	if errorRate >= 50 {
		severity = alerts.SeverityCritical
	} else if errorRate >= 30 {
		severity = alerts.SeverityWarning
	} else {
		severity = alerts.SeverityOK
	}

	key := alerts.AlertKey{Job: m.Name(), Entity: "system", Metric: "system_health"}
	details := fmt.Sprintf("Chain: %s\nSuccess: %.1f%%\nFailed: %d/%d\nConsecutive errors: %d\nLast success: %s",
		m.chain.Name, 100-errorRate, len(errors), tokenCount, consecutiveErr, lastSuccess.Format("15:04:05"))

	m.alertManager.Observe(ctx, key, severity, errorRate, "", details, false, "")
}

func registerOraclePolicies(alertManager *alerts.Manager, cfg *config.OracleConfig, chainID string) {
	jobName := fmt.Sprintf("oracle_%s", chainID)

	// Stablecoin policy
	stableDynamic := make([]alerts.DynamicCooldown, len(cfg.Stablecoin.DynamicCooldowns))
	for i, dc := range cfg.Stablecoin.DynamicCooldowns {
		stableDynamic[i] = alerts.DynamicCooldown{
			Threshold: dc.ThresholdPercent,
			Cooldown:  time.Duration(dc.CooldownSeconds) * time.Second,
		}
	}

	alertManager.RegisterPolicy(jobName, "price_deviation_stable", alerts.AlertPolicy{
		MinValueChange:        cfg.Stablecoin.MinValueChangePercent,
		CooldownWarning:       time.Duration(cfg.Stablecoin.CooldownWarningMinutes) * time.Minute,
		CooldownCritical:      time.Duration(cfg.Stablecoin.CooldownCriticalMinutes) * time.Minute,
		DynamicCooldowns:      stableDynamic,
		ConsecutiveOKRequired: cfg.Stablecoin.ConsecutiveOKRequired,
	})

	// Volatile policy
	volatileDynamic := make([]alerts.DynamicCooldown, len(cfg.Volatile.DynamicCooldowns))
	for i, dc := range cfg.Volatile.DynamicCooldowns {
		volatileDynamic[i] = alerts.DynamicCooldown{
			Threshold: dc.ThresholdPercent,
			Cooldown:  time.Duration(dc.CooldownSeconds) * time.Second,
		}
	}

	alertManager.RegisterPolicy(jobName, "price_deviation_volatile", alerts.AlertPolicy{
		MinValueChange:        cfg.Volatile.MinValueChangePercent,
		CooldownWarning:       time.Duration(cfg.Volatile.CooldownWarningMinutes) * time.Minute,
		CooldownCritical:      time.Duration(cfg.Volatile.CooldownCriticalMinutes) * time.Minute,
		DynamicCooldowns:      volatileDynamic,
		ConsecutiveOKRequired: cfg.Volatile.ConsecutiveOKRequired,
	})

	alertManager.RegisterPolicy(jobName, "system_health", alerts.AlertPolicy{
		MinValueChange:        10.0,
		CooldownWarning:       15 * time.Minute,
		CooldownCritical:      5 * time.Minute,
		ReminderInterval:      30 * time.Minute,
		ConsecutiveOKRequired: 1,
	})
}
