package domainfx

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/yurykabanov/backuper/pkg/domain"
)

func LoadRules(v *viper.Viper) ([]domain.Rule, error) {
	var rules []domain.Rule

	err := v.UnmarshalKey("rules", &rules)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to unmarshal rules")
	}

	return rules, nil
}
