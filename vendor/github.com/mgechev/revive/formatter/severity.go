package formatter

import "github.com/mgechev/revive/lint"

func severity(config lint.RulesConfig, failure lint.Failure) lint.Severity {
	if config, ok := config[failure.RuleName]; ok && config.Severity == lint.SeverityError {
		return lint.SeverityError
	}
	return lint.SeverityWarning
}
