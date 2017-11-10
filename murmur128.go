package murmur3

import (
	"fmt"
	"unsafe"
)

const (
	c1_128 = 0x87c37b91114253d5
	c2_128 = 0x4cf5ad432745937f
)

// Digest128 is a murmur3 128 bit digest that can be written to
// as many consequent times as necessary without heap allocations.
type Digest128 struct {
	digest
	h1 uint64 // Unfinalized running hash part 1.
	h2 uint64 // Unfinalized running hash part 2.
}

// New128 returns a new 128 bit digest.
func New128() Digest128 {
	return New128WithSeed(0)
}

// New128WithSeed returns a new 128 bit digest with a seed.
func New128WithSeed(seed uint32) Digest128 {
	s := uint64(seed)
	return Digest128{digest: digest{seed: seed}, h1: s, h2: s}
}

// Size returns the number of bytes Sum will return.
func (d Digest128) Size() int {
	return 16
}

// Sum returns the current binary hash appended to input byte ref.
func (d Digest128) Sum(b []byte) []byte {
	h1, h2 := d.Sum128()
	return append(b,
		byte(h1>>56), byte(h1>>48), byte(h1>>40), byte(h1>>32),
		byte(h1>>24), byte(h1>>16), byte(h1>>8), byte(h1),

		byte(h2>>56), byte(h2>>48), byte(h2>>40), byte(h2>>32),
		byte(h2>>24), byte(h2>>16), byte(h2>>8), byte(h2),
	)
}

func (d Digest128) bmix(p []byte) (Digest128, []byte) {
	h1, h2 := d.h1, d.h2

	nblocks := len(p) / 16
	for i := 0; i < nblocks; i++ {
		t := (*[2]uint64)(unsafe.Pointer(&p[i*16]))
		k1, k2 := t[0], t[1]

		k1 *= c1_128
		k1 = (k1 << 31) | (k1 >> 33) // rotl64(k1, 31)
		k1 *= c2_128
		h1 ^= k1

		h1 = (h1 << 27) | (h1 >> 37) // rotl64(h1, 27)
		h1 += h2
		h1 = h1*5 + 0x52dce729

		k2 *= c2_128
		k2 = (k2 << 33) | (k2 >> 31) // rotl64(k2, 33)
		k2 *= c1_128
		h2 ^= k2

		h2 = (h2 << 31) | (h2 >> 33) // rotl64(h2, 31)
		h2 += h1
		h2 = h2*5 + 0x38495ab5
	}
	d.h1, d.h2 = h1, h2
	return d, p[nblocks*d.Size():]
}

func (d Digest128) bmixbuf() Digest128 {
	h1, h2 := d.h1, d.h2

	if d.tail != d.Size() {
		panic(fmt.Errorf("expected full block"))
	}

	k1, k2 := d.loadUint64(0), d.loadUint64(1)

	k1 *= c1_128
	k1 = (k1 << 31) | (k1 >> 33) // rotl64(k1, 31)
	k1 *= c2_128
	h1 ^= k1

	h1 = (h1 << 27) | (h1 >> 37) // rotl64(h1, 27)
	h1 += h2
	h1 = h1*5 + 0x52dce729

	k2 *= c2_128
	k2 = (k2 << 33) | (k2 >> 31) // rotl64(k2, 33)
	k2 *= c1_128
	h2 ^= k2

	h2 = (h2 << 31) | (h2 >> 33) // rotl64(h2, 31)
	h2 += h1
	h2 = h2*5 + 0x38495ab5

	d.h1, d.h2 = h1, h2
	d.tail = 0
	return d
}

// Sum128 returns the 128 bit hash of the digest.
func (d Digest128) Sum128() (h1, h2 uint64) {
	h1, h2 = d.h1, d.h2

	var k1, k2 uint64
	switch d.tail & 15 {
	case 15:
		k2 ^= uint64(d.buf[14]) << 48
		fallthrough
	case 14:
		k2 ^= uint64(d.buf[13]) << 40
		fallthrough
	case 13:
		k2 ^= uint64(d.buf[12]) << 32
		fallthrough
	case 12:
		k2 ^= uint64(d.buf[11]) << 24
		fallthrough
	case 11:
		k2 ^= uint64(d.buf[10]) << 16
		fallthrough
	case 10:
		k2 ^= uint64(d.buf[9]) << 8
		fallthrough
	case 9:
		k2 ^= uint64(d.buf[8]) << 0

		k2 *= c2_128
		k2 = (k2 << 33) | (k2 >> 31) // rotl64(k2, 33)
		k2 *= c1_128
		h2 ^= k2

		fallthrough

	case 8:
		k1 ^= uint64(d.buf[7]) << 56
		fallthrough
	case 7:
		k1 ^= uint64(d.buf[6]) << 48
		fallthrough
	case 6:
		k1 ^= uint64(d.buf[5]) << 40
		fallthrough
	case 5:
		k1 ^= uint64(d.buf[4]) << 32
		fallthrough
	case 4:
		k1 ^= uint64(d.buf[3]) << 24
		fallthrough
	case 3:
		k1 ^= uint64(d.buf[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint64(d.buf[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint64(d.buf[0]) << 0
		k1 *= c1_128
		k1 = (k1 << 31) | (k1 >> 33) // rotl64(k1, 31)
		k1 *= c2_128
		h1 ^= k1
	}

	h1 ^= uint64(d.clen)
	h2 ^= uint64(d.clen)

	h1 += h2
	h2 += h1

	h1 = fmix64(h1)
	h2 = fmix64(h2)

	h1 += h2
	h2 += h1

	return h1, h2
}

// Write will write bytes to the digest and return a new digest
// representing the derived digest.
func (d Digest128) Write(p []byte) Digest128 {
	n := len(p)
	d.clen += n

	if d.tail > 0 {
		// Stick back pending bytes.
		nfree := d.Size() - d.tail // nfree ∈ [1, d.Size()-1].
		if len(p) <= nfree {
			// Everything can fit in buf
			for i := 0; i < len(p); i++ {
				d.buf[d.tail] = p[i]
				d.tail++
			}
			if len(p) == int(nfree) {
				d = d.bmixbuf()
			}
			return d
		}

		// One full block can be formed.
		add := p[:nfree]
		for i := 0; i < len(add); i++ {
			d.buf[d.tail] = add[i]
			d.tail++
		}

		// Process the full block
		d = d.bmixbuf()

		p = p[nfree:]
	}

	d, tail := d.bmix(p)

	// Keep own copy of the 0 to Size()-1 pending bytes.
	d.tail = len(tail)
	for i := 0; i < d.tail; i++ {
		d.buf[i] = tail[i]
	}

	return d
}

func fmix64(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return k
}

// Sum128 returns the MurmurHash3 sum of data without any heap allocations.
func Sum128(data []byte) (h1 uint64, h2 uint64) {
	return Sum128WithSeed(data, 0)
}

// Sum128WithSeed returns the MurmurHash3 sum of data given a seed
// without any heap allocations.
func Sum128WithSeed(data []byte, seed uint32) (h1 uint64, h2 uint64) {
	return New128WithSeed(seed).Write(data).Sum128()
}
