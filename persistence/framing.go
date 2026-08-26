package persistence

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
)

// Frame encodes an EventEnvelope as a length-prefixed, checksummed record that
// can be appended to the event log and replayed later. Corruption, truncation
// or sequence gaps are detected during replay rather than silently skipped.
//
// Layout: [8-byte length][4-byte CRC32 of payload][payload]
type frameCodec struct{}

var codec = frameCodec{}

// Encode serializes the envelope payload and wraps it with length and checksum.
func (frameCodec) Encode(env EventEnvelope) ([]byte, error) {
	payload, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	sum := crc32.ChecksumIEEE(payload)
	buf := make([]byte, 8+4+len(payload))
	binary.BigEndian.PutUint64(buf[0:8], uint64(len(payload)))
	binary.BigEndian.PutUint32(buf[8:12], sum)
	copy(buf[12:], payload)
	return buf, nil
}

// Decode parses a single framed record from data at the given offset and
// returns the envelope plus the number of bytes consumed.
func (frameCodec) Decode(data []byte, offset int) (EventEnvelope, int, error) {
	var env EventEnvelope
	if len(data)-offset < 12 {
		return env, 0, fmt.Errorf("truncated header at offset %d", offset)
	}
	length := int(binary.BigEndian.Uint64(data[offset : offset+8]))
	want := binary.BigEndian.Uint32(data[offset+8 : offset+12])
	if length < 0 || offset+12+length > len(data) {
		return env, 0, fmt.Errorf("truncated payload at offset %d (len %d)", offset, length)
	}
	payload := data[offset+12 : offset+12+length]
	if crc32.ChecksumIEEE(payload) != want {
		return env, 0, fmt.Errorf("checksum mismatch at offset %d", offset)
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return env, 0, err
	}
	return env, 12 + length, nil
}
