# okmain-go

Go implementation of the Okmain color extraction algorithm, ported from [okmain-rust](https://github.com/si14/okmain).

## Overview

Extracts dominant colors from images using K-means clustering in Oklab color space. The algorithm is deterministic and produces identical results to the Rust reference implementation when given the same input pixels.

## Features

- Pure Go (no CGO required)
- Supports formats available in Go standard library and imported packages
- File inputs are identified by MIME sniffing instead of file extension
- Multiple input methods: raw RGB buffer, Go `image.Image`, and file path
- Deterministic output with fixed RNG seed

## Supported Image Formats

### Built-in (standard library + pure-Go)

| Format | Extensions | Notes |
|--------|------------|-------|
| JPEG   | .jpg, .jpeg | Decoded via `image/jpeg` |
| PNG    | .png       | Exact match with Rust reference |
| GIF    | .gif       | Standard library `image/gif` |
| WebP   | .webp      | Decoded via `golang.org/x/image/webp` |

### Optional (via `golang.org/x/image`)

To add support for additional formats, import their decoder packages in your code:

```bash
go get golang.org/x/image/bmp
go get golang.org/x/image/tiff
go get golang.org/x/image/ico
```

```go
import (
    _ "image/jpeg"
    _ "image/png"
    _ "image/gif"
    // Optional: import additional format decoders
    // _ "golang.org/x/image/bmp"
    // _ "golang.org/x/image/tiff"
    // _ "golang.org/x/image/ico"
)
```

| Format | Extensions | Import Path |
|--------|------------|-------------|
| BMP    | .bmp       | `golang.org/x/image/bmp` |
| TIFF   | .tiff, .tif | `golang.org/x/image/tiff` |
| ICO    | .ico       | `golang.org/x/image/ico` |

## Installation

```bash
go get github.com/xyxu/okmain-go
```

## Building

```bash
go build ./...
```

## Usage

### Basic: Extract colors from a file

```go
package main

import (
    "fmt"
    "log"

    okmain "github.com/xyxu/okmain-go"
)

func main() {
    input, err := okmain.NewInputImageFromFile("image.jpg")
    if err != nil {
        log.Fatal(err)
    }

    colors := okmain.Colors(input)

    for _, c := range colors {
        fmt.Println(c.Hex())  // e.g., "#ff5733"
    }
}
```

`NewInputImageFromFile` reads the file header to detect the real MIME type and uses the appropriate decoder. Images are processed in their original format without conversion.

### From raw RGB buffer

```go
// RGBRGBRGB... format
buf := []uint8{255, 0, 0, 0, 255, 0, 0, 0, 255}  // 3 pixels
input, err := okmain.NewInputImage(3, 1, buf)
if err != nil {
    log.Fatal(err)
}
colors := okmain.Colors(input)
```

### From Go image.Image

```go
import (
    "image"
    _ "image/jpeg"
    _ "image/png"
    _ "image/gif"
    _ "golang.org/x/image/webp"
    "os"
)

f, _ := os.Open("image.webp")
img, _, _ := image.Decode(f)
f.Close()

input, err := okmain.NewInputImageFromImage(img)
```

If you decode WebP yourself before calling `NewInputImageFromImage`, prefer `golang.org/x/image/webp`:

```go
import (
    "os"

    xwebp "golang.org/x/image/webp"
)

f, _ := os.Open("image.webp")
img, _ := xwebp.Decode(f)
f.Close()

input, err := okmain.NewInputImageFromImage(img)
```

## Configuration

No configuration options are required. The algorithm uses sensible defaults.

## Algorithm Details

1. **Sampling**: Downsample large images to max 250,000 pixels using block averaging
2. **Color Space**: Convert sRGB to Oklab for perceptually uniform clustering
3. **K-means**: Adaptive K-means (K=1..4) with K-means++ initialization
4. **Scoring**: Combine pixel count (center-weighted) and chroma for final ranking
5. **Output**: Up to 4 dominant colors sorted by importance

## Testing

```bash
# Run all tests
go test ./...

# Benchmark
go test -bench=. ./...
```

## Compatibility with Rust Reference

| Input Type | Matches Rust |
|------------|--------------|
| Raw RGB buffer | ✅ Exact |
| PNG file | ✅ Exact |
| GIF file | ✅ Exact |
| WebP file | ✅ Exact |
| JPEG file | ⚠️ Close (standard decoder differences) |

The standard JPEG decoder may produce slightly different pixel values compared to the Rust reference implementation, resulting in minor variations in extracted colors. File-path inputs are matched by MIME content, not by filename extension.

## Project Structure

```
okmain-go/
├── okmain.go              # Public API facade
├── okmain_test.go         # Unit tests
├── internal/
│   ├── conversion/        # RGB/Oklab color space conversions
│   │   ├── conversion.go
│   │   └── helpers.go
│   ├── sampling/          # Image sampling and block averaging
│   │   └── sampling.go
│   ├── kmeans/            # Adaptive K-means clustering
│   │   └── kmeans.go
│   └── rng/               # Deterministic Xoshiro256++ PRNG
│       └── rng.go
├── cmd/                   # Example commands
│   └── check/
│       └── main.go
└── go.mod
```

## License

Same as okmain-rust (see original repository).
