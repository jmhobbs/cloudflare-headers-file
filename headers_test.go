package headers_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	headers "github.com/jmhobbs/cloudflare-headers-file"
)

// Tests derived from the language on: https://developers.cloudflare.com/pages/configuration/headers/

/*
Header rules are defined in multi-line blocks. The first line of a block is the URL or URL pattern where the rule’s headers should be applied. On the next line, an indented list of header names and header values must be written:
*/
func Test_Parse(t *testing.T) {
	r := strings.NewReader(`# This is a comment
/secure/page
  X-Frame-Options: DENY
  X-Content-Type-Options: nosniff
  Referrer-Policy: no-referrer

/static/*
  Access-Control-Allow-Origin: *
  X-Robots-Tag: nosnippet

https://myproject.pages.dev/*
  X-Robots-Tag: noindex
`)
	file, err := headers.Parse(r)
	assert.NoError(t, err)

	assert.Equal(t, headers.File{
		headers.Rule{url.URL{Path: "/secure/page"}, []headers.Header{
			{Name: "X-Frame-Options", Value: "DENY"},
			{Name: "X-Content-Type-Options", Value: "nosniff"},
			{Name: "Referrer-Policy", Value: "no-referrer"},
		}},
		headers.Rule{
			url.URL{Path: "/static/*", RawPath: "/static/*"}, []headers.Header{
				{Name: "Access-Control-Allow-Origin", Value: "*"},
				{Name: "X-Robots-Tag", Value: "nosnippet"},
			}},
		headers.Rule{url.URL{Scheme: "https", Host: "myproject.pages.dev", Path: "/*", RawPath: "/*"}, []headers.Header{
			{Name: "X-Robots-Tag", Value: "noindex"},
		}},
	}, *file)
}

/*
A project is limited to 100 header rules. Each line in the _headers file has a 2,000 character limit. The entire line, including spacing, header name, and value, counts towards this limit.
*/
// TODO: Not implemented

/*
Using absolute URLs is supported, though be aware that absolute URLs must begin with https and specifying a port is not supported.
*/
func Test_Parse_Absolute(t *testing.T) {
	tests := []struct {
		name     string
		rule     string
		hasError bool
	}{
		{
			"valid",
			"https://myproject.pages.dev/*",
			false,
		},
		{
			"invalid scheme",
			"http://myproject.pages.dev/*",
			true,
		},
		{
			"invalid port",
			"https://myproject.pages.dev:1234/*",
			true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := strings.NewReader(test.rule + "\n\tx-should-not: be-parsed")
			_, err := headers.Parse(r)
			if test.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

/*
Cloudflare Pages ignores the incoming request’s port and protocol when matching against an incoming request.
*/
func Test_AbsoluteURL(t *testing.T) {
	r := strings.NewReader(`https://example.com/*
  X-Frame-Options: DENY
`)
	file, err := headers.Parse(r)
	assert.NoError(t, err)

	tests := []struct {
		name     string
		inputURL string
		expected []string
	}{
		{
			"matched",
			"https://example.com/secure/page",
			[]string{
				"X-Frame-Options: DENY",
			},
		},
		{
			"port",
			"https://example.com:1234/secure/page",
			[]string{
				"X-Frame-Options: DENY",
			},
		},
		{
			"scheme",
			"other://example.com/secure/page",
			[]string{
				"X-Frame-Options: DENY",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input, err := url.Parse(test.inputURL)
			assert.NoError(t, err)

			out := file.Match(*input)
			assert.ElementsMatch(t, test.expected, out)
		})
	}
}

/*
You may wish to remove a header which has been added by a more pervasive rule. This can be done by prepending an exclamation mark !.
*/
func Test_Parse_Detach(t *testing.T) {
	r := strings.NewReader(`/*
  Content-Security-Policy: default-src 'self';

/*.jpg
  ! Content-Security-Policy`)
	file, err := headers.Parse(r)
	assert.NoError(t, err)

	assert.EqualValues(t, headers.File{
		headers.Rule{url.URL{Path: "/*", RawPath: "/*"}, []headers.Header{
			{Name: "Content-Security-Policy", Value: "default-src 'self';"},
		}},
		headers.Rule{url.URL{Path: "/*.jpg", RawPath: "/*.jpg"}, []headers.Header{
			{Name: "Content-Security-Policy", Detach: true},
		}},
	}, *file)
}

func Test_File_Match_Detach(t *testing.T) {
	r := strings.NewReader(`/*
  Content-Security-Policy: default-src 'self';

/*.jpg
  ! Content-Security-Policy
`)
	file, err := headers.Parse(r)
	assert.NoError(t, err)

	input, err := url.Parse("https://custom.domain/any/path/image.jpg")
	assert.NoError(t, err)

	out := file.Match(*input)
	assert.Equal(t, []string{}, out)
}

/*
An incoming request which matches multiple rules’ URL patterns will inherit all rules’ headers.
*/
func Test_File_Match_Basic(t *testing.T) {
	r := strings.NewReader(`# This is a comment
/secure/page
  X-Frame-Options: DENY
  X-Content-Type-Options: nosniff
  Referrer-Policy: no-referrer

/static/*
  Access-Control-Allow-Origin: *
  X-Robots-Tag: nosnippet

https://myproject.pages.dev/*
  X-Robots-Tag: noindex
`)
	file, err := headers.Parse(r)
	assert.NoError(t, err)

	tests := []struct {
		name     string
		inputURL string
		expected []string
	}{
		{
			"path",
			"http://example.com/secure/page",
			[]string{
				"X-Frame-Options: DENY",
				"X-Content-Type-Options: nosniff",
				"Referrer-Policy: no-referrer",
			},
		},
		{
			"splat",
			"https://custom.domain/static/image.jpg",
			[]string{
				"Access-Control-Allow-Origin: *",
				"X-Robots-Tag: nosnippet",
			},
		},
		{
			"host",
			"https://myproject.pages.dev/home",
			[]string{
				"X-Robots-Tag: noindex",
			},
		},
		{
			"host and path",
			"https://myproject.pages.dev/secure/page",
			[]string{
				"X-Frame-Options: DENY",
				"X-Content-Type-Options: nosniff",
				"Referrer-Policy: no-referrer",
				"X-Robots-Tag: noindex",
			},
		},
		{
			"host and splat",
			"https://myproject.pages.dev/static/styles.css",
			[]string{
				"Access-Control-Allow-Origin: *",
				"X-Robots-Tag: nosnippet,noindex",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input, err := url.Parse(test.inputURL)
			assert.NoError(t, err)

			out := file.Match(*input)
			assert.ElementsMatch(t, test.expected, out)
		})
	}
}

/*
A placeholder can be defined with :placeholder_name. A colon (:) followed by a letter indicates the start of a placeholder and the placeholder name that follows must be composed of alphanumeric characters and underscores (:[A-Za-z]\w*). Every named placeholder can only be referenced once. Placeholders match all characters apart from the delimiter, which when part of the host, is a period (.) or a forward-slash (/) and may only be a forward-slash (/) when part of the path.
*/
func Test_File_Match_Placeholders(t *testing.T) {
	r := strings.NewReader(`/movies/:title
  x-movie-name: You are watching ":title"

https://:subdomain.example.com/*
  x-subdomain: :subdomain

/double/:ref
  x-ref: :ref and :ref

https://:subdomain.example.dev/*
  x-subdomain: :subdomain and :subdomain
`)
	file, err := headers.Parse(r)
	assert.NoError(t, err)

	tests := []struct {
		name     string
		inputURL string
		expected []string
	}{
		{
			name:     "path",
			inputURL: "https://example.com/movies/star-wars",
			expected: []string{"x-movie-name: You are watching \"star-wars\""},
		},
		{
			name:     "non-greedy path",
			inputURL: "https://example.com/movies/star-wars/episode-1",
			expected: []string{},
		},
		{
			name:     "non-greedy domain",
			inputURL: "https://sub.dub.example.com/whatever",
			expected: []string{},
		},
		{
			name:     "domain",
			inputURL: "https://custom.example.com/whatever",
			expected: []string{"x-subdomain: custom"},
		},
		// TODO: The wording "Every named placeholder can only be referenced once." is ambiguous.
		// Confirm via testing with Cloudflare directly.
		{
			name:     "path double",
			inputURL: "https://example.dev/double/123",
			expected: []string{"x-ref: 123 and :ref"},
		},
		{
			name:     "domain double",
			inputURL: "https://sub.example.dev/whatever",
			expected: []string{"x-subdomain: sub and :subdomain"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input, err := url.Parse(test.inputURL)
			assert.NoError(t, err)

			out := file.Match(*input)
			assert.ElementsMatch(t, test.expected, out)
		})
	}
}

func Test_File_Match_InvalidPlaceholder(t *testing.T) {
	r := strings.NewReader(`/secure/:1page
  x-placeholder: :1page

https://subdomain.:1domain.com/*
  x-placeholder: :1domain
`)
	file, err := headers.Parse(r)
	assert.NoError(t, err)

	input, err := url.Parse("https://subdomain.example.com/secure/example")
	assert.NoError(t, err)

	out := file.Match(*input)
	assert.ElementsMatch(t, []string{}, out)
}
