package processing

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

func pngTestChunk(kind string, data []byte) []byte {
	raw := make([]byte, len(data)+12)
	binary.BigEndian.PutUint32(raw, uint32(len(data)))
	copy(raw[4:], kind)
	copy(raw[8:], data)
	binary.BigEndian.PutUint32(raw[len(raw)-4:], crc32.ChecksumIEEE(raw[4:len(raw)-4]))
	return raw
}

func insertPNGTestChunks(t *testing.T, input []byte, additional ...[]byte) []byte {
	t.Helper()
	chunks, err := readOptimizationPNGChunks(input)
	if err != nil {
		t.Fatal(err)
	}
	output := append(append([]byte(nil), optimizationPNGSignature...), chunks[0].raw...)
	for _, raw := range additional {
		output = append(output, raw...)
	}
	for _, chunk := range chunks[1:] {
		output = append(output, chunk.raw...)
	}
	return output
}

func addPNGProfileFixtures(t *testing.T, fixtures map[string][]byte) {
	profile := syntheticRGBProfile()
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	_, _ = writer.Write(profile)
	_ = writer.Close()
	fixtures["icc-profile"] = insertPNGTestChunks(t, fixtures["rgb"], pngTestChunk("iCCP", append([]byte("Fabricum test RGB\x00\x00"), compressed.Bytes()...)))
	fixtures["srgb"] = insertPNGTestChunks(t, fixtures["rgb"], pngTestChunk("sRGB", []byte{0}))
	gamma := make([]byte, 4)
	binary.BigEndian.PutUint32(gamma, 45455)
	chromaticities := make([]byte, 32)
	for i, value := range []uint32{31270, 32900, 64000, 33000, 30000, 60000, 15000, 6000} {
		binary.BigEndian.PutUint32(chromaticities[i*4:], value)
	}
	fixtures["gamma-chromaticities"] = insertPNGTestChunks(t, fixtures["rgb"], pngTestChunk("gAMA", gamma), pngTestChunk("cHRM", chromaticities), pngTestChunk("sBIT", []byte{8, 8, 8}), pngTestChunk("bKGD", []byte{0, 0, 0, 0, 0, 80}))
	fixtures["hdr"] = insertPNGTestChunks(t, fixtures["rgb"], pngTestChunk("cICP", []byte{9, 16, 0, 1}), pngTestChunk("mDCV", make([]byte, 24)), pngTestChunk("cLLI", make([]byte, 8)))
	fixtures["safe-private-metadata"] = insertPNGTestChunks(t, fixtures["rgb"], pngTestChunk("vpAg", []byte{1, 2, 3}))
	fixtures["grayscale-alpha"] = rawPNGTestFixture(4, 8, false)
	fixtures["interlaced-rgba"] = rawPNGTestFixture(6, 8, true)
	fixtures["rgb-trns"] = insertPNGTestChunks(t, fixtures["rgb"], pngTestChunk("tRNS", []byte{0, 0, 0, 0, 0, 80}))
}

func rawPNGTestFixture(kind, depth byte, interlaced bool) []byte {
	const size = 32
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header, size)
	binary.BigEndian.PutUint32(header[4:], size)
	header[8], header[9] = depth, kind
	passes := [][4]int{{0, 0, 1, 1}}
	if interlaced {
		header[12] = 1
		passes = [][4]int{{0, 0, 8, 8}, {4, 0, 8, 8}, {0, 4, 4, 8}, {2, 0, 4, 4}, {0, 2, 2, 4}, {1, 0, 2, 2}, {0, 1, 1, 2}}
	}
	var raw bytes.Buffer
	for _, pass := range passes {
		for y := pass[1]; y < size; y += pass[3] {
			raw.WriteByte(0)
			for x := pass[0]; x < size; x += pass[2] {
				if kind == 4 {
					raw.Write([]byte{byte(x * 7), byte((x % 3) * 127)})
				} else {
					raw.Write([]byte{byte(x * 7), byte(y * 7), 123, byte((x % 3) * 127)})
				}
			}
		}
	}
	var compressed bytes.Buffer
	writer, _ := zlib.NewWriterLevel(&compressed, zlib.NoCompression)
	_, _ = writer.Write(raw.Bytes())
	_ = writer.Close()
	output := append(append([]byte(nil), optimizationPNGSignature...), pngTestChunk("IHDR", header)...)
	output = append(output, pngTestChunk("IDAT", compressed.Bytes())...)
	return append(output, pngTestChunk("IEND", nil)...)
}

// A generated ICC v2 RGB matrix profile with D50 PCS, gamma curves and the
// required description/copyright tags. No externally licensed profile fixture.
func syntheticRGBProfile() []byte {
	xyz := func(x, y, z uint32) []byte {
		data := make([]byte, 20)
		copy(data, "XYZ ")
		binary.BigEndian.PutUint32(data[8:], x)
		binary.BigEndian.PutUint32(data[12:], y)
		binary.BigEndian.PutUint32(data[16:], z)
		return data
	}
	description := make([]byte, 12+len("Fabricum RGB\x00")+78)
	copy(description, "desc")
	binary.BigEndian.PutUint32(description[8:], uint32(len("Fabricum RGB\x00")))
	copy(description[12:], "Fabricum RGB\x00")
	curve := []byte{'c', 'u', 'r', 'v', 0, 0, 0, 0, 0, 0, 0, 1, 2, 51}
	tags := []struct {
		name string
		data []byte
	}{
		{"desc", description}, {"cprt", append([]byte("text\x00\x00\x00\x00"), []byte("Fabricum synthetic fixture\x00")...)},
		{"wtpt", xyz(63190, 65536, 54061)}, {"rXYZ", xyz(28579, 14582, 913)},
		{"gXYZ", xyz(25236, 46981, 6364)}, {"bXYZ", xyz(9375, 3973, 46784)},
		{"rTRC", curve}, {"gTRC", curve}, {"bTRC", curve},
	}
	profile := make([]byte, 132+len(tags)*12)
	binary.BigEndian.PutUint32(profile[8:], 0x02100000)
	copy(profile[12:], "mntrRGB XYZ ")
	for i, value := range []uint16{2026, 10, 7, 0, 0, 0} {
		binary.BigEndian.PutUint16(profile[24+i*2:], value)
	}
	copy(profile[36:], "acsp")
	copy(profile[40:], "MSFT")
	copy(profile[68:], xyz(63190, 65536, 54061)[8:])
	binary.BigEndian.PutUint32(profile[128:], uint32(len(tags)))
	for i, tag := range tags {
		entry := profile[132+i*12:]
		copy(entry, tag.name)
		binary.BigEndian.PutUint32(entry[4:], uint32(len(profile)))
		binary.BigEndian.PutUint32(entry[8:], uint32(len(tag.data)))
		profile = append(profile, tag.data...)
		for len(profile)%4 != 0 {
			profile = append(profile, 0)
		}
	}
	binary.BigEndian.PutUint32(profile, uint32(len(profile)))
	return profile
}
