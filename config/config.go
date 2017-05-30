package config

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/go-ini/ini"
)

var globalConfig *File

// A File is a representation of an ini file with some custom convenience
// methods.
type File struct {
	instance *ini.File
	Path     string
}

// New reads the file in configPath and returns a corresponding *File
// or an error if encountered.  This File is set as the default active
// config file.
func New(configPath string) (*File, error) {
	config, err := ini.Load(configPath)
	if err != nil {
		return nil, err
	}
	globalConfig = &File{instance: config, Path: configPath}
	return globalConfig, nil
}

// NewIfExists works as New, but does not return an error if the file does not
// exist. Instead, it returns a null File pointer.
func NewIfExists(configPath string) (*File, error) {
	config, err := New(configPath)
	if terr, ok := err.(*os.PathError); ok {
		if terr, ok := terr.Err.(syscall.Errno); ok && terr == syscall.ENOENT {
			return nil, nil
		}
	}
	return config, err
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
func (c *File) GetStrArray(section, name, sep string) ([]string, error) {
	if exists := c.instance.Section(section).HasKey(name); !exists {
		return []string{}, fmt.Errorf("missing `%s` value in [%s] section", name, section)
	}

	value := c.instance.Section(section).Key(name).String()
	if value == "" {
		return []string{}, nil
	}
	return strings.Split(value, sep), nil
}

// GetSection is a convenience method to return an entire section of ini config
func (c *File) GetSection(key string) (*ini.Section, error) {
	return c.instance.GetSection(key)
}
