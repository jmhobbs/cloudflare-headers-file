package headers

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

// Header is a header to apply when a rule is matched
type Header struct {
	Name   string
	Value  string
	Detach bool
}

// Rule is a pattern to match against, and the headers to apply if matched.
type Rule struct {
	Pattern url.URL
	Headers []Header
}

// File is a collection of Rule to match against.
type File []Rule

// Parse the _headers file data from the input reader into rules.
func Parse(in io.Reader) (*File, error) {
	hmap := File{}

	var (
		err     error
		pattern *url.URL
		headers []Header = []Header{}
	)

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Ignore blank lines and comments
		if trimmed == "" || trimmed[0] == '#' {
			continue
		}

		// headers are indented
		if line[0] == '\t' || line[0] == ' ' {
			// if we don't have an open patttern, a header is invalid
			if pattern == nil {
				return nil, fmt.Errorf("header without pattern: %q", line)
			}

			// detach header
			if trimmed[0] == '!' {
				headers = append(headers, Header{Name: strings.TrimSpace(trimmed[1:]), Detach: true})
			} else {
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("invalid header: %q", line)
				}
				headers = append(headers, Header{Name: parts[0], Value: strings.TrimSpace(parts[1])})
			}
		} else {
			if pattern != nil {
				hmap = append(hmap, Rule{*pattern, headers})
			}

			// absolute url pattern
			if submatches := absoluteUrlMatcher.FindStringSubmatch(trimmed); submatches != nil {
				host := submatches[1]
				if hostPortMatcher.MatchString(host) {
					return nil, fmt.Errorf("invalid port in rule: %q", trimmed)
				}
				pattern, err = url.Parse(strings.Replace(trimmed, host, "PLACEHOLDER", 1))
				if err != nil {
					return nil, err
				}
				pattern.Host = host
			} else {
				// non-absolute url pattern (or invalid scheme)
				pattern, err = url.Parse(trimmed)
				if err != nil {
					return nil, err
				}
			}
			if pattern.Scheme != "" && pattern.Scheme != "https" {
				return nil, fmt.Errorf("invalid scheme: %q", pattern.Scheme)
			}
			headers = []Header{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if pattern != nil {
		hmap = append(hmap, Rule{*pattern, headers})
	}

	return &hmap, nil
}

// Match all the rules against the input URL, returning the headers to apply.
func (h File) Match(in url.URL) []string {
	headerStack := []Header{}

	for _, mapping := range h {
		hostname := in.Hostname()

		// If host is set, it must match in some form
		if mapping.Pattern.Host != "" {
			if ok, replacement := hasSplat(mapping.Pattern.Host, hostname, "."); ok {
				headerStack = append(headerStack, replacedHeaders(mapping.Headers, ":splat", replacement)...)
				continue
			}

			if ok, pattern, replacement := hasPlaceholder(mapping.Pattern.Host, hostname, "."); ok {
				headerStack = append(headerStack, replacedHeaders(mapping.Headers, pattern, replacement)...)
				continue
			}

			if mapping.Pattern.Host == hostname {
				headerStack = append(headerStack, mapping.Headers...)
				continue
			}
			continue
		}

		// If the pattern path contains a splat, then see if it matches
		if ok, replacement := hasSplat(mapping.Pattern.Path, in.Path, "/"); ok {
			headerStack = append(headerStack, replacedHeaders(mapping.Headers, ":splat", replacement)...)
			continue
		}

		// If the pattern contains a :placeholder, then see if it matches
		if ok, placeholder, replacement := hasPlaceholder(mapping.Pattern.Path, in.Path, "/"); ok {
			headerStack = append(headerStack, replacedHeaders(mapping.Headers, placeholder, replacement)...)
			continue
		}

		if mapping.Pattern.Path == in.Path {
			headerStack = append(headerStack, mapping.Headers...)
			continue
		}
	}

	return Flatten(headerStack)
}

// Flatten headers into header strings.
func Flatten(headers []Header) []string {
	headersOut := make(map[string][]string)
	for _, header := range headers {
		if header.Detach {
			delete(headersOut, header.Name)
			continue
		}
		if _, ok := headersOut[header.Name]; !ok {
			headersOut[header.Name] = []string{}
		}
		headersOut[header.Name] = append(headersOut[header.Name], header.Value)
	}

	out := []string{}
	for name, values := range headersOut {
		out = append(out, fmt.Sprintf("%s: %s", name, strings.Join(values, ",")))
	}

	return out
}

var (
	placeholderMatcher *regexp.Regexp = regexp.MustCompile(":[A-Za-z][[:word:]]*")
	absoluteUrlMatcher *regexp.Regexp = regexp.MustCompile("^https?://(.*?)/")
	hostPortMatcher    *regexp.Regexp = regexp.MustCompile(":[0-9]+$")
)

func hasPlaceholder(src, in, disallowed string) (bool, string, string) {
	if placeholder := placeholderMatcher.FindString(src); placeholder != "" {
		chunks := strings.SplitN(src, placeholder, 2)
		if strings.HasPrefix(in, chunks[0]) && strings.HasSuffix(in, chunks[1]) {
			replacement := strings.TrimPrefix(strings.TrimSuffix(in, chunks[1]), chunks[0])
			if !strings.Contains(replacement, disallowed) {
				return true, placeholder, replacement
			}
		}
	}
	return false, "", ""
}

func hasSplat(src, in, disallowed string) (bool, string) {
	if strings.Contains(src, "*") {
		chunks := strings.Split(src, "*")
		if strings.HasPrefix(in, chunks[0]) && strings.HasSuffix(in, chunks[1]) {
			replacing := strings.TrimPrefix(strings.TrimSuffix(in, chunks[1]), chunks[0])
			if !strings.Contains(replacing, disallowed) {
				return true, replacing
			}
		}
	}
	return false, ""
}

func replacedHeaders(headers []Header, placeholder, replacement string) []Header {
	out := []Header{}
	for _, header := range headers {
		out = append(out, Header{
			Name:   header.Name,
			Value:  strings.Replace(header.Value, placeholder, replacement, 1),
			Detach: header.Detach,
		})
	}
	return out
}
