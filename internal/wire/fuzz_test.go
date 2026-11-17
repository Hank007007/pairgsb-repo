package wire

import "testing"

// FuzzDecode feeds arbitrary byte sequences through the whole pipeline.
// Every outcome must be either a valid frame or a sentinel error; a panic,
// out-of-range access, or stack blowup fails the run.
func FuzzDecode(f *testing.F) {
	seeds := [][]byte{
		nil,
		{},
		{0x44, 0x46, 0x52, 0x4d},
		{0x44, 0x46, 0x52, 0x4d, 0x01, 0x00},
		{0xff, 0xff, 0xff, 0xff, 0x1f},
		{0x44, 0x46, 0x52, 0x4d, 0x01, 0xff, 0xff, 0xff, 0xff, 0x1f},
		{0x44, 0x46, 0x52, 0x4d, 0x02, 0x00},
		{0x44, 0x46, 0x52, 0x4d, 0x01, 0x01, 0x04},
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, buf []byte) {
		frame, err := Decode(buf)
		if err != nil && frame != nil {
			t.Fatalf("non-nil frame returned with error %v", err)
		}

		// Exercise the lower-level cursor on the same bytes as well.
		r := NewReader(buf)
		for r.Remaining() > 0 {
			if _, err := r.ReadUVarint(); err != nil {
				break
			}
			if _, err := r.ReadByte(); err != nil {
				break
			}
		}
	})
}
