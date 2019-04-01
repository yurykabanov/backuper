package configfx

import (
	"path"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	EnvPrefix              = "backuper"
	DefaultConfigDirectory = "backuper"
	DefaultConfigFile      = "backuper"
)

var (
	defaultConfigPaths = []string{
		".",
		"./config",
		path.Join("/etc", DefaultConfigDirectory),
	}
)

func ViperProvider(logger *logrus.Logger, flagSet *pflag.FlagSet) (*viper.Viper, error) {
	v := viper.New()
	err := v.BindPFlags(flagSet)
	if err != nil {
		return nil, err
	}

	v.AutomaticEnv()
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetConfigName(DefaultConfigFile)

	// Read config from config file
	if configFile := v.GetString("config"); configFile != "" {
		// If user do specify config file, then this file MUST exist and be valid
		// so missing file is a fatal error

		v.SetConfigFile(configFile)

		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	} else {
		// If user does not specify config file, then we'll still try to find appropriate config,
		// but missing file is not an error

		v.SetConfigName(DefaultConfigFile)

		for _, dir := range defaultConfigPaths {
			v.AddConfigPath(dir)
		}

		if err := v.ReadInConfig(); err != nil {
			logger.WithError(err).Warn("Couldn't read config file")
		}
	}

	return v, nil
}
