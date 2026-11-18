package shard

// Arithmetic in GF(2^8) using the AES irreducible polynomial x^8+x^4+x^3+x+1.
// Addition and subtraction are the same operation: XOR. The logarithm tables
// are built from the generator 3 (x+1), which is primitive for this field.

var gfExp [512]byte
var gfLog [256]byte

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		gfExp[i] = byte(x)
		gfLog[byte(x)] = byte(i)
		x = xtime(x) ^ x
	}
	for i := 255; i < len(gfExp); i++ {
		gfExp[i] = gfExp[i-255]
	}
}

func xtime(x int) int {
	x <<= 1
	if x&0x100 != 0 {
		x ^= 0x11b
	}
	return x
}

func mul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

func div(a, b byte) byte {
	if a == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+255-int(gfLog[b])]
}
