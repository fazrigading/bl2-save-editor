package viewer

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

const triJSON = `{"asset":{"version":"2.0"},"scenes":[{"nodes":[0]}],"nodes":[{"mesh":0}],"meshes":[{"primitives":[{"attributes":{"POSITION":0},"indices":1}]}],"buffers":[{"byteLength":42}],"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":36},{"buffer":0,"byteOffset":36,"byteLength":6}],"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},{"bufferView":1,"componentType":5123,"count":3,"type":"SCALAR"}]}`

var triBin = []byte{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 128, 63, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 128, 63, 0, 0, 0, 0,
	0, 0, 1, 0, 2, 0,
}

// writeGLB assembles a minimal binary glTF: header + JSON chunk + BIN chunk.
func writeGLB(t *testing.T, path string) {
	t.Helper()
	jsonPadded := triJSON
	for len(jsonPadded)%4 != 0 {
		jsonPadded += " "
	}
	binPadded := append(append([]byte{}, triBin...), 0, 0)
	var buf bytes.Buffer
	total := 12 + 8 + len(jsonPadded) + 8 + len(binPadded)
	for _, v := range []uint32{0x46546C67, 2, uint32(total)} {
		if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []uint32{uint32(len(jsonPadded)), 0x4E4F534A} {
		if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	buf.WriteString(jsonPadded)
	for _, v := range []uint32{uint32(len(binPadded)), 0x004E4942} {
		if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	buf.Write(binPadded)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseModelGltf(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tri.gltf")
	if err := os.WriteFile(path, []byte(triJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := parseModel(path); err != nil {
		t.Fatalf("parseModel gltf: %v", err)
	}
}

func TestParseModelGlb(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tri.glb")
	writeGLB(t, path)
	if _, err := parseModel(path); err != nil {
		t.Fatalf("parseModel glb: %v", err)
	}
}

func TestParseModelMissing(t *testing.T) {
	if _, err := parseModel(filepath.Join(t.TempDir(), "nope.gltf")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
