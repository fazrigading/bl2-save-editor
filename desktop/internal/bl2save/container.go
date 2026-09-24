package bl2save

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"hash/crc32"
)

var (
	errSaveInvalid      = errors.New("invalid save file")
	errSaveCON          = errors.New("save is an Xbox CON package; extract SaveGame.sav first")
	errSaveVersion      = errors.New("unknown save version")
	errSaveCRC          = errors.New("save CRC check failed")
	errSaveTruncated    = errors.New("save file truncated")
	errSaveVerification = errors.New("save verification failed")
)

// UnwrapPlayerData extracts the serialized protobuf player blob from raw save
// bytes: SHA-1 check, LZO1X decompress, WSG header parse, Huffman decompress,
// CRC check. Port of BaseApp.unwrap_player_data.
func UnwrapPlayerData(data []byte) ([]byte, error) {
	if len(data) >= 4 && string(data[:4]) == "CON " {
		return nil, errSaveCON
	}
	if len(data) < 21 {
		return nil, errSaveTruncated
	}
	sum := sha1.Sum(data[20:])
	if string(data[:20]) != string(sum[:]) {
		return nil, errSaveInvalid
	}

	padded := make([]byte, 0, len(data))
	padded = append(padded, 0xf0)
	padded = append(padded, data[20:]...)
	decompressed, err := lzo1xDecompress(padded)
	if err != nil {
		return nil, err
	}
	if len(decompressed) < 19 {
		return nil, errSaveTruncated
	}
	wsg := decompressed[4:7]
	if string(wsg) != "WSG" {
		return nil, errSaveInvalid
	}
	version := binary.BigEndian.Uint32(decompressed[7:11])
	var crc, size uint32
	if version == 2 {
		crc = binary.BigEndian.Uint32(decompressed[11:15])
		size = binary.BigEndian.Uint32(decompressed[15:19])
	} else if version == 0x02000000 {
		crc = binary.LittleEndian.Uint32(decompressed[11:15])
		size = binary.LittleEndian.Uint32(decompressed[15:19])
	} else {
		return nil, errSaveVersion
	}

	bs := &readBitstream{s: decompressed[19:]}
	tree, err := readHuffTree(bs)
	if err != nil {
		return nil, err
	}
	player, err := huffDecompress(tree, bs, int(size))
	if err != nil {
		return nil, err
	}
	if crc32.ChecksumIEEE(player) != crc {
		return nil, errSaveCRC
	}
	return player, nil
}

// WrapPlayerData serializes the player blob back into save bytes: CRC,
// Huffman bitstream, WSG header, LZO1X compress, SHA-1 prefix. Port of the
// save_io.py write path (little-endian header fields, version 2).
func WrapPlayerData(player []byte) []byte {
	crc := crc32.ChecksumIEEE(player)

	bs := newWriteBitstream()
	tree := makeHuffTree(player)
	writeHuffTree(tree, bs)
	encoding := make(map[byte]huffCode)
	invertHuffTree(tree, 0, 0, encoding)
	huffCompress(encoding, player, bs)
	data := append(bs.getvalue(), 0, 0, 0, 0)

	header := make([]byte, 0, 19)
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)+15))
	header = append(header, lenBuf[:]...)
	header = append(header, 'W', 'S', 'G')
	binary.LittleEndian.PutUint32(lenBuf[:], 2)
	header = append(header, lenBuf[:]...)
	binary.LittleEndian.PutUint32(lenBuf[:], crc)
	header = append(header, lenBuf[:]...)
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(player)))
	header = append(header, lenBuf[:]...)

	compressed, err := lzo1x1Compress(append(header, data...))
	if err != nil {
		panic(err)
	}
	compressed = compressed[1:]

	out := make([]byte, 0, len(compressed)+20)
	sum := sha1.Sum(compressed)
	out = append(out, sum[:]...)
	out = append(out, compressed...)
	return out
}
