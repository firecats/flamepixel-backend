package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type iopair struct {
	inx      int
	iny      int
	outbytes []byte
}

var iopairs = []iopair{
	{0, 19, []byte{0x01, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
	{4, 19, []byte{0, 0, 0x20, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
}

/*
Test plan:
- Test corners of corner panels (ABGH) <= WIP
- Test all on, all off
- Test 1,1 on each panel
*/

func TestLinesToBytes(t *testing.T) {
	for _, iopair := range iopairs {
		lines := ConstructLines(iopair.inx, iopair.iny)
		bytes, _ := linesToBytes(lines, 20, 10)
		assert.Equal(t, iopair.outbytes, bytes, "outbytes should be the same")
	}
}

func ConstructLines(x int, y int) []string {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "0000000000"
	}

	r := []rune(lines[y])
	r[x] = '1'
	lines[y] = string(r)

	return lines
}
