# cardinality-limit-filter

Bounds the number of distinct label sets a metric can carry by dropping or aggregating
label sets that exceed a configured limit.

## Usage

```go
package main

import (
	"fmt"
	"log"

	cardinalitylimitfilter "cardinality-limit-filter"
)

func main() {
	f, err := cardinalitylimitfilter.New(2, cardinalitylimitfilter.Aggregate, []byte("other"))
	if err != nil {
		log.Fatal(err)
	}

	for _, labels := range [][]byte{[]byte("a"), []byte("b"), []byte("c")} {
		out, ok := f.Allow(labels)
		fmt.Printf("%q -> %q (allowed=%v)\n", labels, out, ok)
	}
	// "a" -> "a" (allowed=true)
	// "b" -> "b" (allowed=true)
	// "c" -> "other" (allowed=true)
}
```

## Why this exists

Metrics systems often key series by their full label set. If an application emits labels
with unbounded values (user IDs, request IDs, raw paths), the number of series grows without
limit and so does memory. This library caps that growth at a known number of distinct label
sets. The trade-off is deliberate: you lose per-label detail for the tail, but you keep a
bounded, predictable memory footprint. Choose `Drop` when the tail is noise and `Aggregate`
when you still want the tail counted under one bucket.

## What you will hit

`Allow` always returns a copy of the label bytes, never the caller's slice. That keeps the
filter's internal map key stable even if the caller reuses a buffer, but it means you should
not compare the returned slice by pointer identity. A `nil` input is stored and returned as
`nil`; an empty non-nil input is stored and returned as an empty non-nil slice. They are
treated as two distinct label sets.
