package config

import (
	"net/url"
	"strings"

	"github.com/stretchr/testify/assert"

	"testing"

	"github.com/go-ini/ini"
)

func getURL(f *ini.File) (*url.URL, error) {
	conf := File{
		f,
		"some/path",
	}
	m, _ := conf.GetSection("Main")
	p := getProxySettings(m)
	return p.URL()
}

func TestGetProxySettings(t *testing.T) {
	assert := assert.New(t)

	f, _ := ini.Load([]byte("[Main]\n\nproxy_host = myhost"))

	s, err := getURL(f)
	assert.Nil(err)
	assert.Equal("http://myhost:3128", s.String())

	f, _ = ini.Load([]byte("[Main]\n\nproxy_host = http://myhost"))

	s, err = getURL(f)
	assert.Nil(err)
	assert.Equal("http://myhost:3128", s.String())

	f, _ = ini.Load([]byte("[Main]\n\nproxy_host = https://myhost"))

	s, err = getURL(f)
	assert.Nil(err)
	assert.Equal("https://myhost:3128", s.String())

	// generic user name
	f, _ = ini.Load([]byte(strings.Join([]string{
		"[Main]",
		"proxy_host = https://myhost",
		"proxy_port = 3129",
		"proxy_user = aaditya",
	}, "\n")))

	s, err = getURL(f)
	assert.Nil(err)

	assert.Equal("https://aaditya@myhost:3129", s.String())

	// special char in user name <3
	f, _ = ini.Load([]byte(strings.Join([]string{
		"[Main]",
		"proxy_host = myhost",
		"proxy_port = 3129",
		"proxy_user = léo",
	}, "\n")))

	s, err = getURL(f)
	assert.Nil(err)

	// user is url-encoded and decodes to original string
	assert.Equal("http://l%C3%A9o@myhost:3129", s.String())
	assert.Equal("léo", s.User.Username())

	// generic  user-pass
	f, _ = ini.Load([]byte(strings.Join([]string{
		"[Main]",
		"proxy_host = myhost",
		"proxy_port = 3129",
		"proxy_user = aaditya",
		"proxy_password = password_12",
	}, "\n")))

	s, err = getURL(f)
	assert.Nil(err)
	assert.Equal("http://aaditya:password_12@myhost:3129", s.String())

	// user-pass with schemed host
	f, _ = ini.Load([]byte(strings.Join([]string{
		"[Main]",
		"proxy_host = https://myhost",
		"proxy_port = 3129",
		"proxy_user = aaditya",
		"proxy_password = password_12",
	}, "\n")))

	s, err = getURL(f)
	assert.Nil(err)
	assert.Equal("https://aaditya:password_12@myhost:3129", s.String())

	// special characters in password
	f, _ = ini.Load([]byte(strings.Join([]string{
		"[Main]",
		"proxy_host = https://myhost",
		"proxy_port = 3129",
		"proxy_user = aaditya",
		"proxy_password = /:!?&=@éÔγλῶσσα",
	}, "\n")))

	s, err = getURL(f)
	assert.Nil(err)

	// password is url-encoded and decodes to the original string
	assert.Equal("https://aaditya:%2F%3A%21%3F&=%40%C3%A9%C3%94%CE%B3%CE%BB%E1%BF%B6%CF%83%CF%83%CE%B1@myhost:3129", s.String())

	pass, _ := s.User.Password()
	assert.Equal("/:!?&=@éÔγλῶσσα", pass)
}
