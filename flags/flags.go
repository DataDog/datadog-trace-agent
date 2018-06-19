package flags

import "flag"

var (
	ConfigFile       string
	LegacyConfigFile string
	PIDFilepath      string
	LogLevel         string
	Version          bool
	Info             bool
	CPUProfile       string
	MemProfile       string
)

// Win holds a set of flags which will be populated only during the Windows build.
var Win = struct {
	InstallService   bool
	UninstallService bool
	StartService     bool
	StopService      bool
}{}

func Parse() {
	flag.StringVar(&ConfigFile, "config", defaultConfigFile, "Datadog Agent config file location")
	flag.StringVar(&PIDFilepath, "pid", "", "Path to set pidfile for process")
	flag.BoolVar(&Version, "version", false, "Show version information and exit")
	flag.BoolVar(&Info, "info", false, "Show info about running trace agent process and exit")

	// profiling
	flag.StringVar(&CPUProfile, "cpuprofile", "", "Write cpu profile to file")
	flag.StringVar(&MemProfile, "memprofile", "", "Write memory profile to `file`")

	registerOSSpecificFlags()

	flag.Parse()
}

// IsSet reports whether the flag with the given name was set to any other
// value than its default.
func IsSet(flagName string) bool {
	fs := flag.Lookup(flagName)
	if fs == nil {
		return false
	}
	return fs.DefValue != fs.Value.String()
}
