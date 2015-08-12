package config

import (
	// stdlib
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/vaughan0/go-ini"
)

var globalConfig *File

// A File is a representation of an ini file with some custom convenience
// methods.
type File struct {
	instance ini.File
	Path     string
}

// New reads the file in configPath and returns a corresponding *File
// or an error if encountered.  This File is set as the default active
// config file.
func New(configPath string) (*File, error) {
	config, err := ini.LoadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read config file at %v, err=%v", configPath, err)
	}
	globalConfig = &File{instance: config, Path: configPath}
	return globalConfig, nil
}

// FromReader reads a config from an io.Reader, returning a corresponding
// *File or an error if encountered.  This File is set as the default
// active config "file"
func FromReader(r io.Reader) (*File, error) {
	config, err := ini.Load(r)
	if err != nil {
		return nil, fmt.Errorf("unable to read config: %s", err)
	}
	globalConfig = &File{instance: config, Path: "<mem>"}
	return globalConfig, nil
}

// NewLocal reads the file in path and returns a corresponding *File
// or an error if encountered.  Will not set "default" active config.
func NewLocal(path string) (*File, error) {
	cfg, err := ini.LoadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to load config from %s: %s", path, err)
	}
	return &File{instance: cfg, Path: path}, nil
}

// NewLocalFromReader reads from an io.Reader, returning a corresponding
// *File or an error if ncountered.  Will not set "default" active config.
func NewLocalFromReader(r io.Reader) (*File, error) {
	cfg, err := ini.Load(r)
	if err != nil {
		return nil, err
	}
	return &File{instance: cfg, Path: "<mem>"}, nil
}

// Get returns the currently active global config (the previous config opened
// via NewFile)
func Get() *File {
	return globalConfig
}

// Set points to the given config as the new global config. This is only used
// for testing.
func Set(config ini.File) {
	globalConfig = &File{instance: config}
}

// Get returns a value from the section/name pair, or an error if it can't be found.
func (c *File) Get(section, name string) (string, error) {
	value, ok := c.instance.Get(section, name)
	if !ok {
		return "", fmt.Errorf("missing `%s` value in [%s] section", name, section)
	}
	return value, nil
}

// GetDefault attempts to get the value in section/name, but returns the default
// if one is not found.
func (c *File) GetDefault(section, name string, defaultVal string) string {
	value, err := c.Get(section, name)
	if err != nil {
		return defaultVal
	}
	return value
}

// GetBool returns a truthy config value. 'true' is considered true, everything
// else false.
func (c *File) GetBool(section, name string, defaultVal bool) bool {
	value, err := c.Get(section, name)
	if err != nil {
		return defaultVal
	}
	return value == "true"
}

// GetInt gets an integer value from section/name, or an error if it is missing
// or cannot be converted to an integer.
func (c *File) GetInt(section, name string) (int, error) {
	valueRaw, ok := c.instance.Get(section, name)
	if !ok {
		return 0, fmt.Errorf("missing `%s` value in [%s] section", name, section)
	}
	value, err := strconv.Atoi(valueRaw)
	if !ok {
		return 0, fmt.Errorf("converting `%v` value (%v) to integer, err=%v", name, valueRaw, err)
	}
	return value, nil
}

// GetIntDefault gets an integer value from section/name, returning defaultVal if
// any kind of error occurs.
func (c *File) GetIntDefault(section, name string, defaultVal int) int {
	value, err := c.GetInt(section, name)
	if err != nil {
		return defaultVal
	}
	return value
}

// GetFloat64 gets an integer value from section/name, or an error if it is missing
// or cannot be converted to an integer.
func (c *File) GetFloat64(section, name string) (float64, error) {
	valueRaw, ok := c.instance.Get(section, name)
	if !ok {
		return 0, fmt.Errorf("missing `%s` value in [%s] section", name, section)
	}
	value, err := strconv.ParseFloat(valueRaw, 64)
	if !ok {
		return 0, fmt.Errorf("converting `%v` value (%v) to float, err=%v", name, valueRaw, err)
	}
	return value, nil
}

// GetFloat64Default gets an integer value from section/name, returning defaultVal if
// any kind of error occurs.
func (c *File) GetFloat64Default(section, name string, defaultVal float64) float64 {
	value, err := c.GetFloat64(section, name)
	if err != nil {
		return defaultVal
	}
	return value
}

// GetStrArray returns the value split across `sep` into an array of strings.
func (c *File) GetStrArray(section, name, sep string) ([]string, error) {
	value, ok := c.instance.Get(section, name)
	if !ok {
		return []string{}, fmt.Errorf("missing `%s` value in [%s] section", name, section)
	}
	return strings.Split(value, sep), nil
}

// GetSection returns the given section.
func (c *File) GetSection(section string) ini.Section {
	return c.instance.Section(section)
}

// SectionNames returns a slice of all section names in the config file.
func (c *File) SectionNames() []string {
	sections := make([]string, 0, len(c.instance))
	for n := range c.instance {
		sections = append(sections, n)
	}
	return sections
}
