package lint

// Arguments is type used for the arguments of a rule.
type Arguments = []interface{}

// RuleConfig is type used for the rule configuration.
type RuleConfig struct {
	Arguments Arguments
	Severity  Severity
}

// RulesConfig defines the config for all rules.
type RulesConfig = map[string]RuleConfig

// Config defines the config of the linter.
type Config struct {
	IgnoreGeneratedHeader bool `toml:"ignoreGeneratedHeader"`
	Confidence            float64
	Severity              Severity
	Rules                 RulesConfig `toml:"rule"`
	ErrorCode             int         `toml:"errorCode"`
	WarningCode           int         `toml:"warningCode"`
}
