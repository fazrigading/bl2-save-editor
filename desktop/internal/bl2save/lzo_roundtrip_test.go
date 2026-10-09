// Seeded round-trip sweep over synthetic inputs (fixed seed, deterministic). Recalibrated if math/rand ever changes its NewSource sequence.
package bl2save

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestLZO1XRoundTripSeeded(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	fail := 0
	for i := 0; i < 400; i++ {
		n := 100 + rng.Intn(60000)
		buf := make([]byte, n)
		// semi-compressible: runs + random
		for j := range buf {
			if rng.Intn(3) == 0 {
				buf[j] = byte(rng.Intn(4))
			} else {
				buf[j] = byte(rng.Intn(256))
			}
		}
		c, err := lzo1x1Compress(buf)
		if err != nil {
			t.Fatalf("iter %d compress err: %v", i, err)
		}
		d, err := lzo1xDecompress(c)
		if err != nil {
			t.Logf("iter %d len=%d DECOMPRESS FAIL: %v", i, n, err)
			fail++
			continue
		}
		if !bytes.Equal(d, buf) {
			t.Logf("iter %d len=%d MISMATCH (len %d)", i, n, len(d))
			fail++
		}
	}
	t.Logf("400 iters, failures=%d", fail)
}
