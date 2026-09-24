package bl2save

import "sort"

type huffNode struct {
	weight int
	leaf   bool
	value  byte
	kids   [2]*huffNode
}

func huffLess(a, b *huffNode) bool {
	if a.weight != b.weight {
		return a.weight < b.weight
	}
	if a.leaf && b.leaf {
		return a.value < b.value
	}
	return a.leaf
}

func makeHuffTree(data []byte) *huffNode {
	var frequencies [256]int
	for _, c := range data {
		frequencies[c]++
	}

	nodes := make([]*huffNode, 0, 256)
	for i, f := range frequencies {
		if f != 0 {
			nodes = append(nodes, &huffNode{weight: f, leaf: true, value: byte(i)})
		}
	}
	sort.Slice(nodes, func(x, y int) bool { return huffLess(nodes[x], nodes[y]) })

	for len(nodes) > 1 {
		left, right := nodes[0], nodes[1]
		nodes = nodes[2:]
		parent := &huffNode{weight: left.weight + right.weight, kids: [2]*huffNode{left, right}}
		idx := sort.Search(len(nodes), func(j int) bool { return huffLess(parent, nodes[j]) })
		nodes = append(nodes, nil)
		copy(nodes[idx+1:], nodes[idx:])
		nodes[idx] = parent
	}
	return nodes[0]
}

func readHuffTree(b *readBitstream) (*huffNode, error) {
	nodeType, err := b.readBit()
	if err != nil {
		return nil, err
	}
	if nodeType == 0 {
		left, err := readHuffTree(b)
		if err != nil {
			return nil, err
		}
		right, err := readHuffTree(b)
		if err != nil {
			return nil, err
		}
		return &huffNode{kids: [2]*huffNode{left, right}}, nil
	}
	v, err := b.readByte()
	if err != nil {
		return nil, err
	}
	return &huffNode{leaf: true, value: v}, nil
}

func writeHuffTree(node *huffNode, b *writeBitstream) {
	if node.leaf {
		b.writeBit(1)
		b.writeByte(node.value)
		return
	}
	b.writeBit(0)
	writeHuffTree(node.kids[0], b)
	writeHuffTree(node.kids[1], b)
}

type huffCode struct {
	code uint64
	bits int
}

func invertHuffTree(node *huffNode, code uint64, bits int, out map[byte]huffCode) {
	if node.leaf {
		out[node.value] = huffCode{code: code, bits: bits}
		return
	}
	invertHuffTree(node.kids[0], code<<1, bits+1, out)
	invertHuffTree(node.kids[1], code<<1|1, bits+1, out)
}

func huffDecompress(tree *huffNode, b *readBitstream, size int) ([]byte, error) {
	output := make([]byte, 0, size)
	for len(output) < size {
		node := tree
		for {
			bit, err := b.readBit()
			if err != nil {
				return nil, err
			}
			node = node.kids[bit]
			if node == nil {
				return nil, errBitstreamOutOfRange
			}
			if node.leaf {
				output = append(output, node.value)
				break
			}
		}
	}
	return output, nil
}

func huffCompress(encoding map[byte]huffCode, data []byte, b *writeBitstream) {
	for _, c := range data {
		ec := encoding[c]
		b.writeBits(ec.code, ec.bits)
	}
}
