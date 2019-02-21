package domain

import "time"

type Rule struct {
	Name            string        `mapstructure:"name"`
	Image           string        `mapstructure:"image"`
	Command         []string      `mapstructure:"command"`
	TargetDirectory string        `mapstructure:"target_directory"`
	CronSpec        string        `mapstructure:"cron_spec"`
	Timeout         time.Duration `mapstructure:"timeout"`
	PreserveAtMost  int           `mapstructure:"preserve_at_most"`
}
