// +build !appengine
// +build gc
// +build !noasm

package main

const haveAccumulateSIMD = true

//go:noescape
func accumulateSIMD(dst []uint8, src []float32)
