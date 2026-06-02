package pure

import (
	"bytes"
	"context"
	"encoding/json"
)

// UnmarshalJSON parses JSON data into v (standard library delegation).
func UnmarshalJSON(ctx context.Context, data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// MarshalJSON returns the JSON encoding of v using an allocator-backed buffer.
func MarshalJSON(ctx context.Context, v any) ([]byte, error) {
	a, ok := FromContext(ctx)
	if !ok {
		return json.Marshal(v)
	}

	buf := NewBuffer(a)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}

	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}

	// Snapshot into a stable Go-heap slice so the caller is safe after reset.
	return bytes.Clone(b), nil
}
