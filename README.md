# font-go

This is a port of https://github.com/google/font-rs to the Go programming
language.

You can visually inspect rasterization by running:

```
go build && ./font-go
```

and viewing the resultant out.png file.

To run the benchmarks with and without SIMD assembler:

```
$ go test -test.bench=.
$ go test -test.bench=. -tags=noasm
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
