[![Go Reference](https://pkg.go.dev/badge/github.com/jmhobbs/cloudflare-headers-file.svg)](https://pkg.go.dev/github.com/jmhobbs/cloudflare-headers-file)
[![Tests](https://github.com/jmhobbs/cloudflare-headers-file/actions/workflows/test.yaml/badge.svg)](https://github.com/jmhobbs/cloudflare-headers-file/actions/workflows/test.yaml)
[![codecov](https://codecov.io/github/jmhobbs/cloudflare-headers-file/graph/badge.svg?token=BptWbMUx6H)](https://codecov.io/github/jmhobbs/cloudflare-headers-file)

# Cloudflare Headers File

This is a Go implementation of the [Coudflare `_headers` file](https://developers.cloudflare.com/pages/configuration/headers/) that conforms to the description on that docs page.  Any variation is considered a bug, please report it.

## Usage

```
# _headers

/foo/bar
    x-foo: bar
```

```go
package main

import (
	"fmt"
	"net/url"
	"os"

	headers "github.com/jmhobbs/cloudflare-headers-file"
)

func main() {
	f, err := os.Open("_headers")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	h, err := headers.Parse(f)
	if err != nil {
		panic(err)
	}

	input, _ := url.Parse("https://example.com/foo/bar")

	// prints "[x-foo: bar]"
	fmt.Println(h.Match(*input))
}
```
