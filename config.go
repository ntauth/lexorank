package lexorank

// AppendStrategy defines how new keys should be generated when appending
type AppendStrategy int

const (
	// AppendStrategyDefault uses Between(last, TopOf(bucket)) or Between(BottomOf(bucket), first)
	AppendStrategyDefault AppendStrategy = iota

	// AppendStrategyStep uses After(step) for append and Before(step) for prepend.
	AppendStrategyStep
)

// Config holds configuration for the lexorank system
type Config struct {
	// MaxRankLength is the maximum allowed length for ranks (default: 6)
	MaxRankLength int

	// AppendStrategy determines how new keys are generated when appending
	AppendStrategy AppendStrategy

	// StepSize is the distance to use when using AppendStrategyStep
	StepSize int64
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		MaxRankLength:  6,
		AppendStrategy: AppendStrategyDefault,
		StepSize:       1,
	}
}

// WithMaxRankLength sets the maximum rank length
func (c *Config) WithMaxRankLength(length int) *Config {
	newConfig := *c
	newConfig.MaxRankLength = length
	return &newConfig
}

// WithAppendStrategy sets the append strategy
func (c *Config) WithAppendStrategy(strategy AppendStrategy) *Config {
	newConfig := *c
	newConfig.AppendStrategy = strategy
	return &newConfig
}

// WithStepSize sets the step size for step-based append strategies
func (c *Config) WithStepSize(step int64) *Config {
	newConfig := *c
	newConfig.StepSize = step
	return &newConfig
}

// ProductionConfig returns a configuration optimized for production with
// longer ranks and step-based strategies.
func ProductionConfig() *Config {
	return &Config{
		MaxRankLength:  128, // Allow for longer ranks
		AppendStrategy: AppendStrategyStep,
		StepSize:       1000, // Every new key is 1000 steps away from the previous key
	}
}
