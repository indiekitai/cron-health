package cron

import (
	"time"

	"github.com/robfig/cron/v3"
)

// Parser wraps the cron expression parser
type Parser struct {
	parser cron.Parser
}

// New creates a new cron parser with standard cron syntax (5 fields)
func New() *Parser {
	return &Parser{
		parser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
	}
}

// Parse validates a cron expression and returns a schedule
func (p *Parser) Parse(expr string) (cron.Schedule, error) {
	return p.parser.Parse(expr)
}

// NextRun returns the next run time for a cron expression after the given time
func (p *Parser) NextRun(expr string, after time.Time) (time.Time, error) {
	schedule, err := p.parser.Parse(expr)
	if err != nil {
		return time.Time{}, err
	}
	return schedule.Next(after), nil
}

// NextRunNow returns the next run time for a cron expression from now
func (p *Parser) NextRunNow(expr string) (time.Time, error) {
	return p.NextRun(expr, time.Now())
}

// Validate checks if a cron expression is valid
func (p *Parser) Validate(expr string) error {
	_, err := p.parser.Parse(expr)
	return err
}

// DescribeCron returns a human-readable description of a cron expression
func DescribeCron(expr string) string {
	// Simple descriptions for common patterns
	switch expr {
	case "* * * * *":
		return "every minute"
	case "0 * * * *":
		return "every hour"
	case "0 0 * * *":
		return "every day at midnight"
	case "0 0 * * 0":
		return "every Sunday at midnight"
	case "0 0 1 * *":
		return "first day of every month"
	}

	// For other expressions, just return the expression
	return expr
}
