package protocol

import (
	"bytes"
	"encoding/json"
	"sync"
)

// Serializer handles message serialization and deserialization
type Serializer struct {
	bufferPool sync.Pool
}

// NewSerializer creates a new serializer
func NewSerializer() *Serializer {
	return &Serializer{
		bufferPool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

// Marshal serializes a protocol message to JSON
func (s *Serializer) Marshal(msg *ProtocolMessage) ([]byte, error) {
	if msg == nil {
		return nil, ErrInvalidMessage
	}

	// Validate message type
	if msg.Type != TypeReq && msg.Type != TypeRes && msg.Type != TypeEvent {
		return nil, ErrInvalidType
	}

	// For request messages, ensure ID is present
	if msg.Type == TypeReq && msg.ID == "" {
		return nil, ErrMissingID
	}

	// Use buffer pool for better performance
	buf := s.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer s.bufferPool.Put(buf)

	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(msg); err != nil {
		return nil, err
	}

	// Remove trailing newline added by Encode
	data := buf.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}

	return data, nil
}

// Unmarshal deserializes JSON to a protocol message
func (s *Serializer) Unmarshal(data []byte) (*ProtocolMessage, error) {
	if len(data) == 0 {
		return nil, ErrInvalidMessage
	}

	var msg ProtocolMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	// Validate message type
	if msg.Type != TypeReq && msg.Type != TypeRes && msg.Type != TypeEvent {
		return nil, ErrInvalidType
	}

	return &msg, nil
}

// MarshalRequest creates a request message
func (s *Serializer) MarshalRequest(id, method string, params interface{}) ([]byte, error) {
	var paramsRaw json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		paramsRaw = data
	}

	msg := &ProtocolMessage{
		Type:   TypeReq,
		ID:     id,
		Method: method,
		Params: paramsRaw,
	}

	return s.Marshal(msg)
}

// MarshalResponse creates a response message
func (s *Serializer) MarshalResponse(id string, ok bool, payload interface{}, errorMsg string) ([]byte, error) {
	msg := &ProtocolMessage{
		Type:    TypeRes,
		ID:      id,
		Ok:      ok,
		Payload: payload,
	}

	if errorMsg != "" {
		msg.Error = errorMsg
	}

	return s.Marshal(msg)
}

// MarshalEvent creates an event message
func (s *Serializer) MarshalEvent(event string, data interface{}, seq int) ([]byte, error) {
	msg := &ProtocolMessage{
		Type:  TypeEvent,
		Event: event,
		Data:  data,
		Seq:   seq,
	}

	return s.Marshal(msg)
}

// UnmarshalRequest unmarshals a request message's parameters
func (s *Serializer) UnmarshalRequest(msg *ProtocolMessage, params interface{}) error {
	if msg.Type != TypeReq {
		return ErrInvalidType
	}

	if msg.Params == nil {
		return nil // No parameters
	}

	return json.Unmarshal(msg.Params, params)
}

// UnmarshalResponse unmarshals a response message's payload
func (s *Serializer) UnmarshalResponse(msg *ProtocolMessage, payload interface{}) error {
	if msg.Type != TypeRes {
		return ErrInvalidType
	}

	if msg.Payload == nil {
		return nil // No payload
	}

	data, err := json.Marshal(msg.Payload)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, payload)
}

// Global serializer instance
var defaultSerializer = NewSerializer()

// Marshal is a convenience function using the default serializer
func Marshal(msg *ProtocolMessage) ([]byte, error) {
	return defaultSerializer.Marshal(msg)
}

// Unmarshal is a convenience function using the default serializer
func Unmarshal(data []byte) (*ProtocolMessage, error) {
	return defaultSerializer.Unmarshal(data)
}
