package main

import (
	"fmt"
	"github.com/tarm/serial"
	"net"
	"strconv"
	"strings"
)

const START = 0x7f
const STOP = 0x7e
const ESCAPE = 0x20

const BAUD = 115200

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

	//sc := &serial.Config{Name: "/dev/ttyUSB0", Baud: BAUD}
	sc := &serial.Config{Name: "/dev/ttyS0", Baud: BAUD}
	ser, err := serial.OpenPort(sc)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Serving.  Monitor with: cu -l /dev/ttyUSB? -s %d\n", BAUD)
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

	if ver != 0 {
		return nil, fmt.Errorf("Unsupported version %d", ver)
	}
	if cols != 10 {
		return nil, fmt.Errorf("Unsupported width %d", cols)
	}
	if rows != 20 {
		return nil, fmt.Errorf("Unsupported height %d", rows)
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

	return linesToBytes(lines[0:rows], rows, cols)
}

func linesToBytes(lines []string, rows int, cols int) ([]byte, error) {
	// dmxfires wiring:
	// master: 1- 5 -> col1,  6-10 -> col2, 11-15 -> col3
	// slave: 17-21 -> col4, 22-26 -> col5
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
	}

	/*
	   for t := 0; t < len(outbits); t++ {
	       fmt.Printf("%d", outbits[t])
	       if t % 8 == 7 {
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
