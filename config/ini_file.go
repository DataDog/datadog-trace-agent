package config

import (
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/go-ini/ini"
)

var globalConfig *File

// A File is a representation of an ini file with some custom convenience
// methods.
type File struct {
	instance *ini.File
	Path     string
}

// NewIni reads the file in configPath and returns a corresponding *File
// or an error if encountered.  This File is set as the default active
// config file.
func NewIni(configPath string) (*File, error) {
	config, err := ini.Load(configPath)
	if err != nil {
		return nil, err
	}
	globalConfig = &File{instance: config, Path: configPath}
	return globalConfig, nil
}

// Get returns the currently active global config (the previous config opened
// via NewFile)
func Get() *File {
	return globalConfig
}

// Set points to the given config as the new global config. This is only used
// for testing.
func Set(config *ini.File) {
	globalConfig = &File{instance: config}
}

// Get returns a value from the section/name pair, or an error if it can't be found.
func (c *File) Get(section, name string) (string, error) {
	exists := c.instance.Section(section).HasKey(name)
	if !exists {
		return "", fmt.Errorf("missing `%s` value in [%s] section", name, section)
	}
	return c.instance.Section(section).Key(name).String(), nil
}

// GetDefault attempts to get the value in section/name, but returns the default
// if one is not found.
func (c *File) GetDefault(section, name string, defaultVal string) string {
	return c.instance.Section(section).Key(name).MustString(defaultVal)
}

// GetInt gets an integer value from section/name, or an error if it is missing
// or cannot be converted to an integer.
func (c *File) GetInt(section, name string) (int, error) {
	value, err := c.instance.Section(section).Key(name).Int()
	if err != nil {
		return 0, fmt.Errorf("missing `%s` value in [%s] section", name, section)
	}
	return value, nil
}

// GetInt64 gets a 64-bit integer value from section/name, or an error if it is missing
// or cannot be converted to an integer.
func (c *File) GetInt64(section, name string) (int64, error) {
	value, err := c.instance.Section(section).Key(name).Int64()
	if err != nil {
		return 0, fmt.Errorf("missing `%s` value in [%s] section", name, section)
	}
	return value, nil
}

// GetFloat gets an float value from section/name, or an error if it is missing
// or cannot be converted to an float.
func (c *File) GetFloat(section, name string) (float64, error) {
	value, err := c.instance.Section(section).Key(name).Float64()
	if err != nil {
		return 0, fmt.Errorf("missing `%s` value in [%s] section", name, section)
	}
	return value, nil
}

// GetStrArray returns the value split across `sep` into an array of strings.
func (c *File) GetStrArray(section, name string, sep rune) ([]string, error) {
	value, err := c.Get(section, name)

	if err != nil || value == "" {
		return []string{}, err
	}

	return splitString(value, sep)
}

// GetSection is a convenience method to return an entire section of ini config
func (c *File) GetSection(key string) (*ini.Section, error) {
	return c.instance.GetSection(key)
}

func splitString(s string, sep rune) ([]string, error) {
	r := csv.NewReader(strings.NewReader(s))
	r.TrimLeadingSpace = true
	r.LazyQuotes = true
	r.Comma = sep

	record, err := r.Read()

	if err != nil {
		return []string{}, err
	}

	return record, nil
}
