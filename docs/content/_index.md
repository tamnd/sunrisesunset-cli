---
title: "sunrisesunset"
description: "A command line for the Sunrise-Sunset API."
heroTitle: "sunrisesunset, from the command line"
heroLead: "A command line for the Sunrise-Sunset API. One pure-Go binary, no API key, output that pipes into the rest of your tools, and a resource-URI driver other programs can address."
heroPrimaryURL: "/getting-started/quick-start/"
heroPrimaryText: "Get started"
---

`sunrisesunset` reads public sunrisesunset data over plain HTTPS, shapes it into
clean records, and gets out of your way.

```bash
sunrisesunset page <path>            # fetch one page as a record
sunrisesunset page <path> -o json    # as JSON, ready for jq
sunrisesunset links <path>           # the pages it links to, each addressable
sunrisesunset serve --addr :7777     # the same operations over HTTP
```

There is nothing to sign up for and nothing to run alongside it. Output adapts
to where it goes: an aligned table on your terminal, JSONL the moment you pipe
it somewhere.

## Two ways to use it

- **As a command** for reading sunrisesunset by hand or in a script. Start with
  the [quick start](/getting-started/quick-start/).
- **As a resource-URI driver** so a host like
  [ant](https://github.com/tamnd/ant) can address sunrisesunset as
  `sunrisesunset://` URIs and follow links across sites. See
  [resource URIs](/guides/resource-uris/).

Both are the same code: one operation, declared once, is a CLI command, an HTTP
route, an MCP tool, and a URI dereference.

## Where to go next

- New here? Read the [introduction](/getting-started/introduction/), then the
  [quick start](/getting-started/quick-start/).
- Installing? See [installation](/getting-started/installation/).
- Doing a specific job? The [guides](/guides/) are task-first.
- Need every flag? The [CLI reference](/reference/cli/) is the full surface.
