package configfx

import (
	"os"

	"github.com/spf13/pflag"
)

func PFlags() *pflag.FlagSet {
	fs := pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)

	// Config file flag
	fs.StringP("config", "c", "", "Config file")

	return fs
}
