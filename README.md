# font-go

This is a port of https://github.com/google/font-rs to the Go programming
language.

There are two implementations, using fixed and floating point math. The fixed
point implementation benchmarks 1.3 to 1.4 times faster on GOARCH=amd64, but
may have rendering artifacts above 1024 ppem. It uses mostly int32 math,
although some int64 and float32 math is used for numerical accuracy.

You can visually inspect rasterization by running:

```
cd floating
go build && ./floating
```

and viewing the resultant out.png file.

To run the benchmarks with and without SIMD assembler:

```
go test -test.bench=.
go test -test.bench=. -tags=noasm
```

## Authors

The main author is Nigel Tao.

## Contributions

We gladly accept contributions via GitHub pull requests, as long as the author
has signed the Google Contributor License. Please see CONTRIBUTIONS.md for more
details.

### Disclaimer

This is not an official Google product (experimental or otherwise), it is just
code that happens to be owned by Google.
