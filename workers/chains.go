package workers

import (
	"fmt"
	"strings"
)

// ChainID represents supported blockchain networks
type ChainID string

const (
	ChainBase      ChainID = "base"
	ChainOptimism  ChainID = "optimism"
	ChainMoonbeam  ChainID = "moonbeam"
	ChainMoonriver ChainID = "moonriver"
)

// TokenMeta holds metadata for a token on a specific chain
type TokenMeta struct {
	Symbol       string
	MTokAddr     string  // Moonwell mToken contract address
	Decimals     int     // Token decimals
	TableName    string  // Database table name
	IsStablecoin bool    // Whether this is a stablecoin
	PegValue     float64 // Expected peg value for stablecoins
	PriceAddress string  // Underlying token address for price lookups
}

// ChainConfig holds chain-specific configuration
type ChainConfig struct {
	ID            ChainID
	Name          string
	OracleAddress string
	Tokens        map[string]TokenMeta
	PriceNetwork  string
}

// GetChainsByEnv returns enabled chains based on environment configuration
func GetChainsByEnv(enabledChains string) ([]ChainConfig, error) {
	if enabledChains == "" {
		return []ChainConfig{BaseChain()}, nil
	}

	chainIDs := strings.Split(enabledChains, ",")
	configs := make([]ChainConfig, 0, len(chainIDs))

	for _, id := range chainIDs {
		id = strings.TrimSpace(strings.ToLower(id))
		var cfg ChainConfig
		switch ChainID(id) {
		case ChainBase:
			cfg = BaseChain()
		case ChainOptimism:
			cfg = OptimismChain()
		case ChainMoonbeam:
			cfg = MoonbeamChain()
		case ChainMoonriver:
			cfg = MoonriverChain()
		default:
			return nil, fmt.Errorf("unsupported chain: %s", id)
		}
		configs = append(configs, cfg)
	}

	return configs, nil
}

func BaseChain() ChainConfig {
	return ChainConfig{
		ID:            ChainBase,
		Name:          "Base",
		OracleAddress: "0xEC942bE8A8114bFD0396A5052c36027f2cA6a9d0",
		PriceNetwork:  "base-mainnet",
		Tokens:        BaseTokens(),
	}
}

func OptimismChain() ChainConfig {
	return ChainConfig{
		ID:            ChainOptimism,
		Name:          "Optimism",
		OracleAddress: "0x2f1490bD6aD10C9CE42a2829afa13EAc0b746dcf",
		PriceNetwork:  "opt-mainnet",
		Tokens:        OptimismTokens(),
	}
}

func MoonbeamChain() ChainConfig {
	return ChainConfig{
		ID:            ChainMoonbeam,
		Name:          "Moonbeam",
		OracleAddress: "0xED301cd3EB27217BDB05C4E9B820a8A3c8B665f9",
		PriceNetwork:  "moonbeam-mainnet",
		Tokens:        MoonbeamTokens(),
	}
}

func MoonriverChain() ChainConfig {
	return ChainConfig{
		ID:            ChainMoonriver,
		Name:          "Moonriver",
		OracleAddress: "0xED301cd3EB27217BDB05C4E9B820a8A3c8B665f9",
		PriceNetwork:  "moonriver-mainnet",
		Tokens:        MoonriverTokens(),
	}
}
