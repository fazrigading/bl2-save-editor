package bl2save

import (
	"encoding/binary"
	"errors"
)

var errLZOOutOfRange = errors.New("lzo1x data out of range")

var clzTable = [37]uint32{
	32, 0, 1, 26, 2, 23, 27, 0, 3, 16, 24, 30, 28, 11, 0, 13,
	4, 7, 17, 0, 25, 22, 31, 15, 29, 10, 12, 6, 0, 21, 14, 9,
	5, 20, 8, 19, 18,
}

func lzoExpandZeroes(src []byte, ip, extra int) (int, int, error) {
	start := ip
	for ip < len(src) && src[ip] == 0 {
		ip++
	}
	if ip >= len(src) {
		return 0, 0, errLZOOutOfRange
	}
	v := ((ip-start)*255 + int(src[ip])) + extra
	return v, ip + 1, nil
}

func lzoCopyEarlier(b []byte, offset, chunkSize int) []byte {
	i := len(b) - offset
	end := i + chunkSize
	for i < end {
		hi := i + chunkSize
		if hi > len(b) {
			hi = len(b)
		}
		chunk := append([]byte(nil), b[i:hi]...)
		i += len(chunk)
		chunkSize -= len(chunk)
		b = append(b, chunk...)
	}
	return b
}

func lzoReadXor32(src []byte, p1, p2 int) uint32 {
	v1 := binary.LittleEndian.Uint32(src[p1:])
	v2 := binary.LittleEndian.Uint32(src[p2:])
	return v1 ^ v2
}

func lzo1xDecompress(s []byte) ([]byte, error) {
	src := s
	dst := make([]byte, 0, len(src)*2)
	ip := 5

	if ip >= len(src) {
		return nil, errLZOOutOfRange
	}
	t := int(src[ip])
	ip++
	if t > 17 {
		t -= 17
		if ip+t > len(src) {
			return nil, errLZOOutOfRange
		}
		dst = append(dst, src[ip:ip+t]...)
		ip += t
		if ip >= len(src) {
			return nil, errLZOOutOfRange
		}
		t = int(src[ip])
		ip++
	} else if t < 16 {
		if t == 0 {
			var err error
			t, ip, err = lzoExpandZeroes(src, ip, 15)
			if err != nil {
				return nil, err
			}
		}
		if ip+t+3 > len(src) {
			return nil, errLZOOutOfRange
		}
		dst = append(dst, src[ip:ip+t+3]...)
		ip += t + 3
		if ip >= len(src) {
			return nil, errLZOOutOfRange
		}
		t = int(src[ip])
		ip++
	}

	for {
		for {
			if ip >= len(src) {
				return nil, errLZOOutOfRange
			}
			switch {
			case t >= 64:
				if ip >= len(src) {
					return nil, errLZOOutOfRange
				}
				dst = lzoCopyEarlier(dst, 1+((t>>2)&7)+(int(src[ip])<<3), (t>>5)+1)
				ip++
			case t >= 32:
				count := t & 31
				if count == 0 {
					var err error
					count, ip, err = lzoExpandZeroes(src, ip, 31)
					if err != nil {
						return nil, err
					}
				}
				if ip+2 > len(src) {
					return nil, errLZOOutOfRange
				}
				t = int(src[ip])
				dst = lzoCopyEarlier(dst, 1+(int(t)|int(src[ip+1])<<8)>>2, count+2)
				ip += 2
			case t >= 16:
				offset := (t & 8) << 11
				count := t & 7
				if count == 0 {
					var err error
					count, ip, err = lzoExpandZeroes(src, ip, 7)
					if err != nil {
						return nil, err
					}
				}
				if ip+2 > len(src) {
					return nil, errLZOOutOfRange
				}
				t = int(src[ip])
				offset += (int(t) | int(src[ip+1])<<8) >> 2
				ip += 2
				if offset == 0 {
					return dst, nil
				}
				dst = lzoCopyEarlier(dst, offset+0x4000, count+2)
			default:
				if ip >= len(src) {
					return nil, errLZOOutOfRange
				}
				dst = lzoCopyEarlier(dst, 1+(t>>2)+(int(src[ip])<<2), 2)
				ip++
			}

			t &= 3
			if t == 0 {
				break
			}
			if ip+t > len(src) {
				return nil, errLZOOutOfRange
			}
			dst = append(dst, src[ip:ip+t]...)
			ip += t
			if ip >= len(src) {
				return nil, errLZOOutOfRange
			}
			t = int(src[ip])
			ip++
		}

		for {
			if ip >= len(src) {
				return nil, errLZOOutOfRange
			}
			t = int(src[ip])
			ip++
			if t < 16 {
				if t == 0 {
					var err error
					t, ip, err = lzoExpandZeroes(src, ip, 15)
					if err != nil {
						return nil, err
					}
				}
				if ip+t+3 > len(src) {
					return nil, errLZOOutOfRange
				}
				dst = append(dst, src[ip:ip+t+3]...)
				ip += t + 3
				if ip >= len(src) {
					return nil, errLZOOutOfRange
				}
				t = int(src[ip])
				ip++
			}
			if t < 16 {
				if ip >= len(src) {
					return nil, errLZOOutOfRange
				}
				dst = lzoCopyEarlier(dst, 1+0x0800+(t>>2)+(int(src[ip])<<2), 3)
				ip++
				t &= 3
				if t == 0 {
					continue
				}
				if ip+t > len(src) {
					return nil, errLZOOutOfRange
				}
				dst = append(dst, src[ip:ip+t]...)
				ip += t
				if ip >= len(src) {
					return nil, errLZOOutOfRange
				}
				t = int(src[ip])
				ip++
			}
			break
		}
	}
}

func lzo1xCompressCore(src, dst []byte, ti, ipStart, ipLen int) (int, []byte, error) {
	var dictEntries [16384]uint32

	inEnd := ipStart + ipLen
	ipEnd := ipStart + ipLen - 20

	ip := ipStart
	ii := ipStart

	if ti < 4 {
		ip += 4 - ti
	}
	ip += 1 + ((ip - ii) >> 5)
	var mPos int
	for {
		for {
			if ip >= ipEnd {
				return inEnd - (ii - ti), dst, nil
			}
			dv := binary.LittleEndian.Uint32(src[ip:])
			dindex := ((0x1824429D * dv) >> 18) & 0x3FFF
			mPos = ipStart + int(dictEntries[dindex])
			dictEntries[dindex] = uint32(ip-ipStart) & 0xFFFF
			if dv == binary.LittleEndian.Uint32(src[mPos:]) {
				break
			}
			ip += 1 + ((ip - ii) >> 5)
		}

		ii -= ti
		ti = 0
		t := ip - ii
		if t != 0 {
			if t <= 3 {
				dst[len(dst)-2] |= byte(t)
				dst = append(dst, src[ii:ii+t]...)
			} else if t <= 16 {
				dst = append(dst, byte(t-3))
				dst = append(dst, src[ii:ii+t]...)
			} else {
				if t <= 18 {
					dst = append(dst, byte(t-3))
				} else {
					tt := t - 18
					dst = append(dst, 0)
					n := tt / 255
					rem := tt % 255
					for j := 0; j < n; j++ {
						dst = append(dst, 0)
					}
					dst = append(dst, byte(rem))
				}
				dst = append(dst, src[ii:ii+t]...)
				ii += t
			}
		}

		mLen := 4
		v := lzoReadXor32(src, ip+mLen, mPos+mLen)
		if v == 0 {
			for {
				mLen += 4
				v = lzoReadXor32(src, ip+mLen, mPos+mLen)
				if ip+mLen >= ipEnd {
					break
				} else if v != 0 {
					mLen += int(clzTable[(v&-v)%37] >> 3)
					break
				}
			}
		} else {
			mLen += int(clzTable[(v&-v)%37] >> 3)
		}

		mOff := ip - mPos
		ip += mLen
		ii = ip
		if mLen <= 8 && mOff <= 0x0800 {
			mOff--
			dst = append(dst, byte(((mLen-1)<<5)|((mOff&7)<<2)))
			dst = append(dst, byte(mOff>>3))
		} else if mOff <= 0x4000 {
			mOff--
			if mLen <= 33 {
				dst = append(dst, byte(32|(mLen-2)))
			} else {
				mLen -= 33
				dst = append(dst, 32)
				n := mLen / 255
				rem := mLen % 255
				for j := 0; j < n; j++ {
					dst = append(dst, 0)
				}
				dst = append(dst, byte(rem))
			}
			dst = append(dst, byte((mOff<<2)&0xFF))
			dst = append(dst, byte((mOff>>6)&0xFF))
		} else {
			mOff -= 0x4000
			if mLen <= 9 {
				dst = append(dst, byte(0xFF&(16|((mOff>>11)&8)|(mLen-2))))
			} else {
				mLen -= 9
				dst = append(dst, byte(0xFF&(16|((mOff>>11)&8))))
				n := mLen / 255
				rem := mLen % 255
				for j := 0; j < n; j++ {
					dst = append(dst, 0)
				}
				dst = append(dst, byte(rem))
			}
			dst = append(dst, byte((mOff<<2)&0xFF))
			dst = append(dst, byte((mOff>>6)&0xFF))
		}
	}
}

func lzo1x1Compress(s []byte) ([]byte, error) {
	src := s
	dst := make([]byte, 0, len(s)+len(s)/16+64)

	ip := 0
	length := len(s)
	t := 0

	dst = append(dst, 240)
	dst = append(dst, byte(length>>24), byte(length>>16), byte(length>>8), byte(length))

	var err error
	for length > 20 && t+length > 31 {
		ll := length
		if ll > 49152 {
			ll = 49152
		}
		t, dst, err = lzo1xCompressCore(src, dst, t, ip, ll)
		if err != nil {
			return nil, err
		}
		ip += ll
		length -= ll
	}
	t += length

	if t > 0 {
		ii := len(s) - t
		if len(dst) == 5 && t <= 238 {
			dst = append(dst, byte(17+t))
		} else if t <= 3 {
			dst[len(dst)-2] |= byte(t)
		} else if t <= 18 {
			dst = append(dst, byte(t-3))
		} else {
			tt := t - 18
			dst = append(dst, 0)
			n := tt / 255
			rem := tt % 255
			for j := 0; j < n; j++ {
				dst = append(dst, 0)
			}
			dst = append(dst, byte(rem))
		}
		dst = append(dst, src[ii:ii+t]...)
	}

	dst = append(dst, 16|1)
	dst = append(dst, 0)
	dst = append(dst, 0)

	return dst, nil
}
