package domain

import "time"

type Rule struct {
	Name            string         `mapstructure:"name"`
	Image           string         `mapstructure:"image"`
	Command         []string       `mapstructure:"command"`
	TargetDirectory string         `mapstructure:"target_directory"`
	Timeout         time.Duration  `mapstructure:"timeout"`
	CronSpec        string         `mapstructure:"cron_spec"`
	RotationRules   []RotationRule `mapstructure:"rotation_rules"`
	StorageName     string         `mapstructure:"storage_name"`
}

type RotationRule struct {
	Period         time.Duration `mapstructure:"period"`
	PreserveAtMost int           `mapstructure:"preserve_at_most"`
}
