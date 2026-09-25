# aitoolsblocklist-go

Go module for recognising AI tools by domain. Network software written in Go (proxies, DNS forwarders, egress gateways, log processors) can call `Check` and learn whether a hostname belongs to an AI product, what category it falls in, and what the vendor says about training on customer input. The same data powers [DNS filtering with AI threat protection](https://www.aitoolsblocklist.com), refreshed daily.

```bash
go get github.com/explainableaixai/aitoolsblocklist-go
```

The module uses only the standard library.

## Minimal program

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	atb "github.com/explainableaixai/aitoolsblocklist-go"
)

func main() {
	c := atb.New(os.Getenv("AQ_API_KEY"))
	r, err := c.Check(context.Background(), "chat.openai.com")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(r["blocked"], r["primary_category"], r["trains_on_data"])
}
```

## API surface

The package is deliberately small:

- `New(key string) *Client` builds a client with the hosted base URL and an `http.Client` that times out after 30 seconds.
- `(*Client).Check(ctx, domain) (Result, error)` performs one lookup.
- `Result` is a `map[string]any` holding the decoded JSON.
- `*APIError` carries the HTTP `Status` and raw `Body` for any response of 400 or above.

`Client` has three exported fields, `APIKey`, `BaseURL` and `HTTPClient`, which you can change after `New`. Replace `HTTPClient` to add tracing, a custom transport or a different timeout. Point `BaseURL` at a stub in tests.

## Fields in a Result

| Key | Type after JSON decoding | Notes |
|---|---|---|
| `domain` | string | What you asked about |
| `blocked` | bool | True for a known AI tool |
| `matched_domain` | string | Only when a parent domain matched |
| `primary_category` | string | Main function of the tool |
| `ai_type` | string | Whether AI is the product or a feature of it |
| `categories` | []any of map[string]any | `category` and optional `subcategory` |
| `trains_on_data` | string | `yes`, `no`, `opt_out_default` or `unstated` |
| `opt_out_available`, `enterprise_no_training`, `api_no_training` | string | Same value set |
| `terms_checked` | string | Date of the terms review |

Numbers in `map[string]any` decode as `float64`, and booleans as `bool`. Type-assert with the two-value form, `v, ok := r["blocked"].(bool)`, so a missing field never panics.

## Error handling with errors.As

```go
r, err := c.Check(ctx, host)
var apiErr *atb.APIError
switch {
case errors.As(err, &apiErr) && apiErr.Status == 429:
	// back off
case errors.As(err, &apiErr) && (apiErr.Status == 401 || apiErr.Status == 403):
	// key problem or quota exhausted: alert a human
case err != nil:
	// network error, context deadline, or invalid JSON
}
```

Empty key or empty domain returns an ordinary error before any request is made. Response bodies are read up to 2 MiB, which is far more than a lookup ever returns.

## Putting it in the request path

A lookup is a network round trip, so put a cache in front of it. For a proxy that sees the same hosts over and over, a map guarded by a mutex with a fixed lifetime is enough. `golang.org/x/sync/singleflight` prevents a burst of identical lookups when a popular host first appears:

```go
type cachedChecker struct {
	c   *atb.Client
	g   singleflight.Group
	mu  sync.RWMutex
	hit map[string]entry
}

type entry struct {
	r   atb.Result
	exp time.Time
}

func (cc *cachedChecker) Check(ctx context.Context, host string) (atb.Result, error) {
	cc.mu.RLock()
	e, ok := cc.hit[host]
	cc.mu.RUnlock()
	if ok && time.Now().Before(e.exp) {
		return e.r, nil
	}
	v, err, _ := cc.g.Do(host, func() (any, error) {
		return cc.c.Check(ctx, host)
	})
	if err != nil {
		return nil, err
	}
	r := v.(atb.Result)
	cc.mu.Lock()
	cc.hit[host] = entry{r, time.Now().Add(12 * time.Hour)}
	cc.mu.Unlock()
	return r, nil
}
```

Twelve hours keeps answers fresh without flooding the API. Unknown domains are worth caching too. They are the majority of traffic.

## From answer to action

The lookup describes a tool. Your code decides. Keep that decision in one function so it can be reviewed:

```go
func verdict(r atb.Result) string {
	if b, _ := r["blocked"].(bool); !b {
		return "allow"
	}
	switch r["trains_on_data"] {
	case "no":
		return "log"
	case "yes", "unstated":
		return "deny"
	default:
		return "warn"
	}
}
```

Most deployments add an approved-tools list that overrides this, keyed on `matched_domain` when present.

## High-volume networks

A resolver answering thousands of queries a second should not depend on an outside call per query, however well cached. Plans that include the full downloadable list let you load every classified domain into memory at startup and refresh it nightly. Use this module for the gaps: hosts that are new since the last download, or ad-hoc checks from tooling and scripts.

## Context and cancellation

Every call takes a `context.Context`. In a server, pass the request context so an abandoned client connection cancels the lookup. For background jobs, set a deadline:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
r, err := c.Check(ctx, "example.com")
```

The shorter of the context deadline and `HTTPClient.Timeout` wins.

## Testing

`net/http/httptest` makes the client easy to test:

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprint(w, `{"domain":"x.ai","blocked":true,"trains_on_data":"unstated"}`)
}))
defer srv.Close()
c := atb.New("test")
c.BaseURL = srv.URL
```

The module's own test does the same, and checks that the `Accept: application/json` header is sent.

## Related services

- Before choosing rules, run [a free shadow AI audit](https://www.shadowaitools.com/free-shadow-ai-audit.php) on your firewall exports to see which tools are already in use.
- Autonomous agents need a different kind of control: an [AI agent allow list of page types](https://www.aiagentallowlist.com/page-types-database.php).
- For [enterprise web filtering data](https://www.webfilteringdatabase.com) beyond AI, load the general filtering database alongside this one.

## Deploying behind a corporate proxy

If your service reaches the internet through an outbound proxy, set it on the transport rather than in code that builds requests:

```go
c := atb.New(key)
c.HTTPClient = &http.Client{
	Timeout:   10 * time.Second,
	Transport: &http.Transport{Proxy: http.ProxyFromEnvironment},
}
```

`ProxyFromEnvironment` honours `HTTPS_PROXY` and `NO_PROXY`, the same variables your other tools already use.

The register is also reachable from Rust through [the aitoolsblocklist crate](https://crates.io/crates/aitoolsblocklist), from notebooks through [the PyPI release](https://pypi.org/project/aitoolsblocklist/), from Node through [the npm module](https://www.npmjs.com/package/aitoolsblocklist), and from Flutter through [the pub.dev package](https://pub.dev/packages/aitoolsblocklist).

## License

MIT
