package inkfish

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckCredentials(t *testing.T) {
	proxy := &Inkfish{
		Passwd: []UserEntry{
			{
				// $ echo -n "foo" | shasum -a 256
				// 2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae  -
				Username:     "foo",
				PasswordHash: "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
			},
		},
	}
	assert.False(t, proxy.credentialsAreValid("bar", "bar"))
	assert.False(t, proxy.credentialsAreValid("foo", "bar"))
	assert.True(t, proxy.credentialsAreValid("foo", "foo"))
}

func TestParseAclUrl(t *testing.T) {
	aclUrl, err := parseAclEntry([]string{})
	assert.Nil(t, aclUrl)
	assert.NotNil(t, err)

	aclUrl, err = parseAclEntry([]string{"foo", "bar", "baz"})
	assert.Nil(t, aclUrl)
	assert.NotNil(t, err)

	url := `^http://boards\.4chan\.org/b/`

	// 2-form
	// url ^http://boards\.4chan\.org/b/
	aclUrl, err = parseAclEntry([]string{"url", url})
	assert.NotNil(t, aclUrl)
	assert.Nil(t, err)
	assert.Equal(t, true, aclUrl.AllMethods)
	assert.Empty(t, aclUrl.Methods)
	assert.Equal(t, url, aclUrl.Pattern.String())

	// 3-form
	// url GET,POST,HEAD ^http://boards\.4chan\.org/b/
	aclUrl, err = parseAclEntry([]string{"url", "GET,POST,HEAD", url})
	assert.NotNil(t, aclUrl)
	assert.Nil(t, err)
	assert.Equal(t, false, aclUrl.AllMethods)
	assert.Equal(t, []string{"GET", "POST", "HEAD"}, aclUrl.Methods)
	assert.Equal(t, url, aclUrl.Pattern.String())
}

func TestBrokenAclConfigs(t *testing.T) {
	aclConfig, err := parseAcl([]string{
		"klaatu", "barada", "nikto",
	})
	assert.Nil(t, aclConfig)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "line: 1")

	aclConfig, err = parseAcl([]string{
		"from foo",
		"url SOME THING WRONG",
	})
	assert.Nil(t, aclConfig)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "line: 2")
}

func TestAclConfig(t *testing.T) {
	aclConfig, err := parseAcl([]string{
		"from foo",
		"from bar",
		"url ^http(s)?://google.com/",
		"url GET,HEAD ^http(s)?://yahoo.com/",
	})
	assert.NotNil(t, aclConfig)
	assert.Nil(t, err)

	assert.True(t, aclConfig.permits("foo", "GET", "https://google.com/"))
	assert.True(t, aclConfig.permits("bar", "GET", "https://google.com/"))
	assert.False(t, aclConfig.permits("baz", "GET", "https://google.com/"))
	assert.True(t, aclConfig.permits("foo", "GET", "https://yahoo.com/"))
	assert.False(t, aclConfig.permits("foo", "POST", "https://yahoo.com/"))
}

func TestAclConfigWithBypass(t *testing.T) {
	aclConfig, err := parseAcl([]string{
		"from foo",
		"from bar",
		"url ^http(s)?://google.com/",
		"url GET,HEAD ^http(s)?://yahoo.com/",
		"bypass foo.com:443",
		"bypass bar.com:443",
	})
	assert.NotNil(t, aclConfig)
	assert.Nil(t, err)

	assert.True(t, aclConfig.bypassMitm("foo", "foo.com:443"))
	assert.True(t, aclConfig.bypassMitm("foo", "bar.com:443"))
	assert.False(t, aclConfig.bypassMitm("baz", "foo.com:443"))
}

func TestLoadConfig(t *testing.T) {
	proxy := NewInkfish()
	err := proxy.LoadConfigFromDirectory("testdata/unit_test_config")
	assert.NotNil(t, proxy.Acls)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(proxy.Acls))
}
