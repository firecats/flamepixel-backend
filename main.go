package main

import (
	"fmt"
	"github.com/tarm/serial"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const START = 0x7f
const STOP = 0x7e
const ESCAPE = 0x20

const BAUD = 115200

const BLANKTIME = 3

var lastFireTime time.Time
var interstitialId = 0
var r1 *rand.Rand

func main() {
	addr := net.UDPAddr{
		Port: 1075,
		IP:   net.IPv4zero,
	}
	conn, err := net.ListenUDP("udp", &addr)
	defer conn.Close()
	if err != nil {
		panic(err)
	}

	port := os.Args[1]
	sc := &serial.Config{Name: port, Baud: BAUD}
	ser, err := serial.OpenPort(sc)
	if err != nil {
		panic(err)
	}

	lastFireTime = time.Now()

	s1 := rand.NewSource(time.Now().UnixNano())
	r1 = rand.New(s1)

	fmt.Printf("Serving on %s.  Monitor with: cu -l /dev/ttyUSB? -s %d\n", port, BAUD)
	Serve(conn, ser)
}

func Serve(conn *net.UDPConn, ser *serial.Port) {
	for {
		u := make([]byte, 1024)

		n, err := conn.Read(u)
		if err != nil {
			panic(err)
		}

		bytes, err := handleUdp(u, n)
		if err != nil {
			panic(err)
		}

		//bytes[0] = 0xff
		//bytes[1] = 0xff
		//bytes[2] = 0xff
		//bytes[3] = 0xff
		fmt.Printf("Serial send: ")
		for i := 0; i < len(bytes); i++ {
			fmt.Printf("%02x", bytes[i])
		}
		fmt.Printf("\n")

		bf := make([]byte, len(bytes)*2+3)
		bf_i := 0
		/*
		   for b := 0; b < 8; b++ {
		      bf[bf_i] = START
		      bf_i += 1
		   }
		*/

		bf[bf_i] = START
		bf_i += 1

		var csum byte
		for b := 0; b < len(bytes); b++ {
			if bytes[b] == START || bytes[b] == ESCAPE || bytes[b] == STOP {
				fmt.Printf("frame: %d, ESCAPED\n", bf_i)
				bf[bf_i] = ESCAPE
				bf_i += 1
				bf[bf_i] = bytes[b] ^ ESCAPE
			} else {
				bf[bf_i] = bytes[b]
			}
			bf_i += 1
			csum ^= bytes[b]
		}
		bf[bf_i] = csum
		bf_i += 1
		bf[bf_i] = STOP
		bf_i += 1

		fmt.Printf("Framed send: ")
		for i := 0; i < bf_i; i++ {
			fmt.Printf("%02x", bf[i])
		}
		fmt.Printf("\n")

		n, err = ser.Write(bf[0:bf_i])
		if err != nil {
			panic(err)
		}
		if n != bf_i {
			panic(fmt.Errorf("Not enough bytes written: wrote %d, expected %d", n, len(bytes)))
		}
	}
}

func getInt(str string) int {
	val, _ := strconv.ParseInt(str, 0, 32)
	return int(val)
}

func handleUdp(b []byte, n int) ([]byte, error) {
	fmt.Printf("Received %d UDP bytes\n", n)
	parts := strings.SplitN(string(b[0:n]), "\n", 4)

	ver := getInt(parts[0])
	cols := getInt(parts[1])
	rows := getInt(parts[2])
	board := parts[3]

	if ver != 0 && ver != 1 {
		return nil, fmt.Errorf("Unsupported version %d", ver)
	}
	if cols != 10 {
		return nil, fmt.Errorf("Unsupported width %d", cols)
	}
	if rows != 20 {
		return nil, fmt.Errorf("Unsupported height %d", rows)
	}
	if ver == 1 {
		rows += 1
	}

	lines := strings.SplitN(board, "\n", rows+1)
	if len(lines) < rows {
		return nil, fmt.Errorf("Not enough lines: received %d, expected %d", len(lines), rows)
	}

	for l := 0; l < rows; l++ {
		line := lines[l]
		if len(line) != cols {
			return nil, fmt.Errorf("Line %d length wrong: received %d, expected %d", l, len(line), cols)
		}
	}

	if allBlank(lines[0:rows]) {
		blankFor := time.Now().Sub(lastFireTime).Seconds()
		if blankFor >= BLANKTIME {
			fmt.Printf("INTERSTITIAL after %vs\n", blankFor)
			ver = 0
			rows = 20
			if interstitialId == 0 {
				interstitialId = r1.Intn(len(INTERSTITIALS)) + 1
			}
			lines = INTERSTITIALS[interstitialId-1]
		}
	} else {
		lastFireTime = time.Now()
		interstitialId = 0
	}

	if ver == 0 {
		return linesToBytes(lines[0:rows], rows, cols, false)
	} else {
		vp, err := lineToVP(lines[0])
		if err != nil {
			return nil, err
		}
		return linesToBytes(lines[1:rows], rows, cols, vp)
	}
}

func allBlank(lines []string) bool {
	for _, line := range lines {
		if strings.Contains(line, "1") {
			return false
		}
	}
	return true
}

var INTERSTITIALS = [][]string{
	{
		"0100000010",
		"1100000011",
		"1100000011",
		"1110110111",
		"1111111111",

		"1100110011",
		"1100110011",
		"0111111110",
		"0111001110",
		"0011111100",

		"0010000100",
		"0010000100",
		"0000000000",
		"0000000000",
		"0000000000",

		"0000000000",
		"0000000000",
		"0000000000",
		"0000000000",
		"0000000000",
	},
	{
		"1010000000",
		"1010111000",
		"1110101000",
		"0010111000",
		"1110000101",

		"0000000101",
		"0011100111",
		"0010100000",
		"0011101110",
		"0000001010",

		"1111001010",
		"1000000000",
		"1110000000",
		"1001000011",
		"1000011010",

		"1001010011",
		"1001010010",
		"1001010011",
		"0000000000",
		"1111111111",
	},
}

func lineToVP(line string) (bool, error) {
	if strings.Contains(line, "1") {
		return true, nil
	}
	return false, nil
}

func linesToBytes(lines []string, rows int, cols int, vp bool) ([]byte, error) {
	// dmxfires wiring:
	// master: 1- 5 -> col1,  6-10 -> col2, 11-15 -> col3
	// slave: 17-21 -> col4, 22-26 -> col5, 27 -> victory poofer
	// Lower values correspond to bottom solenoids.

	// Panel layout: GH
	//               EF
	//               CD
	//               AB

	// RS-485 output is:
	// start byte
	// 4 bytes per panel (in alphabetical order):
	//   solenoids  1- 8
	//   solenoids  9-16
	//   solenoids 17-25
	//   solenoids 26-32
	// checksum (xor of bits)
	// stop byte
	// PPP escaping is performed (does not affect checksum)

	grid := make([][]uint8, cols)
	for c := 0; c < cols; c++ {
		grid[c] = make([]uint8, rows)
	}

	for l, line := range lines {
		fmt.Printf("%02d: ", l)
		for c, col := range line {
			if col == '0' {
				grid[c][rows-l-1] = 0
				fmt.Printf("-")
			} else {
				grid[c][rows-l-1] = 1
				fmt.Printf("*")
			}
		}
		fmt.Printf("\n")
	}
	if vp {
		fmt.Printf("VICTORY POOF!\n")
	}

	const outlen = 256

	outbits := make([]uint8, outlen)

	for p := 0; p < 8; p++ {
		start_col := (p % 2) * 5
		start_row := (p / 2) * 5
		byte := p * 32

		//fmt.Printf("panel: %d, srow: %02d, scol: %02d, byte: %03d\n", p, start_row, start_col, byte)

		for c := 0; c < 3; c++ {
			for r := 0; r < 5; r++ {
				outbits[byte] = grid[start_col+c][start_row+r]
				byte++
			}
		}

		byte++ // skip over output 16, which doesn't connect anywhere

		for c := 3; c < 5; c++ {
			for r := 0; r < 5; r++ {
				outbits[byte] = grid[start_col+c][start_row+r]
				byte++
			}
		}

		if vp {
			outbits[byte] = 1
		}

	}

	/*
		for t := 0; t < len(outbits); t++ {
			fmt.Printf("%d", outbits[t])
			if t%8 == 7 {
				fmt.Printf("\n")
			}
		}
	*/

	outbytes := make([]byte, outlen/8)

	var b uint
	for b = 0; b < outlen; b++ {
		bi := b % 8
		by := b / 8
		outbytes[by] |= outbits[b] << bi
	}

	return outbytes, nil
}
