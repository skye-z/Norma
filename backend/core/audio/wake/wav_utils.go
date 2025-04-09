package audio

import (
	"encoding/binary"
	"os"
)

func writeWavHeader(f *os.File, sampleRate uint32, dataLen uint32) error {
	byteRate := sampleRate * 2
	blockAlign := uint16(2)
	bitsPerSample := uint16(16)
	riffSize := 36 + dataLen

	header := []any{
		[]byte("RIFF"), riffSize, []byte("WAVE"),
		[]byte("fmt "), uint32(16), uint16(1), uint16(1), sampleRate, byteRate, blockAlign, bitsPerSample,
		[]byte("data"), dataLen,
	}

	for _, h := range header {
		if err := binary.Write(f, binary.LittleEndian, h); err != nil {
			return err
		}
	}
	return nil
}
