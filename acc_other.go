// +build !amd64 appengine !gc noasm

package main

const haveAccumulateSIMD = false

func accumulateSIMD(dst []uint8, src []float32) {}
