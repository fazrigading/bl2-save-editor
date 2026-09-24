package bl2save

import "errors"

var errBitstreamOutOfRange = errors.New("bitstream read out of range")

type readBitstream struct {
	s []byte
	i int
}

func (b *readBitstream) readBit() (int, error) {
	i := b.i
	b.i = i + 1
	if i>>3 >= len(b.s) {
		return 0, errBitstreamOutOfRange
	}
	return int(b.s[i>>3]>>(7-uint(i&7))) & 1, nil
}

func (b *readBitstream) readBits(n int) (uint64, error) {
	if n <= 0 {
		return 0, nil
	}
	if n > 64 {
		return 0, errBitstreamOutOfRange
	}
	i := b.i
	end := i + n
	if (end+7)>>3 > len(b.s) {
		return 0, errBitstreamOutOfRange
	}
	chunk := b.s[i>>3 : (end+7)>>3]
	value := uint64(chunk[0] & (0xFF >> uint(i&7)))
	for _, c := range chunk[1:] {
		value = value<<8 | uint64(c)
	}
	if end&7 != 0 {
		value >>= uint(8 - (end & 7))
	}
	b.i = end
	return value, nil
}

func (b *readBitstream) readByte() (byte, error) {
	i := b.i
	b.i = i + 8
	if i>>3 >= len(b.s) {
		return 0, errBitstreamOutOfRange
	}
	b0 := b.s[i>>3]
	if i&7 == 0 {
		return b0, nil
	}
	if (i>>3)+1 >= len(b.s) {
		return 0, errBitstreamOutOfRange
	}
	b1 := b.s[(i>>3)+1]
	return byte(((uint(b0) << 8) | uint(b1)) >> (8 - uint(i&7)) & 0xFF), nil
}

type writeBitstream struct {
	s []byte
	b byte
	i int
}

func newWriteBitstream() *writeBitstream {
	return &writeBitstream{i: 7}
}

func (w *writeBitstream) writeBit(bit byte) {
	i := w.i
	v := w.b | bit<<uint(i)
	if i == 0 {
		w.s = append(w.s, v)
		w.b = 0
		w.i = 7
	} else {
		w.b = v
		w.i = i - 1
	}
}

func (w *writeBitstream) writeBits(value uint64, n int) {
	acc := uint64(w.b)
	i := w.i
	for n >= i+1 {
		shift := n - (i + 1)
		n = n - (i + 1)
		acc |= value >> uint(shift)
		value &= ^(acc << uint(shift))
		w.s = append(w.s, byte(acc))
		acc = 0
		i = 7
	}
	if n > 0 {
		acc |= value << uint(i+1-n)
		i = i - n
	}
	w.b = byte(acc)
	w.i = i
}

func (w *writeBitstream) writeByte(b byte) {
	i := w.i
	if i == 7 {
		w.s = append(w.s, b)
	} else {
		w.s = append(w.s, w.b|b>>(7-uint(i)))
		w.b = (b << uint(i+1)) & 0xFF
	}
}

func (w *writeBitstream) getvalue() []byte {
	if w.i != 7 {
		out := make([]byte, 0, len(w.s)+1)
		out = append(out, w.s...)
		out = append(out, w.b)
		return out
	}
	return w.s
}
