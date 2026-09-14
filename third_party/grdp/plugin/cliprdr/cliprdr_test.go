package cliprdr

import (
	"encoding/binary"
	"testing"
)

type fakeSender struct{ messages [][]byte }

func (s *fakeSender) SendToChannel(_ string, data []byte) (int, error) {
	copyOfData := append([]byte(nil), data...)
	s.messages = append(s.messages, copyOfData)
	return len(data), nil
}

type fakeClipboard struct {
	text    string
	written string
}

func (c *fakeClipboard) ReadText() (string, error)   { return c.text, nil }
func (c *fakeClipboard) WriteText(text string) error { c.written = text; return nil }

func pdu(msgType, flags uint16, payload []byte) []byte {
	result := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint16(result[0:2], msgType)
	binary.LittleEndian.PutUint16(result[2:4], flags)
	binary.LittleEndian.PutUint32(result[4:8], uint32(len(payload)))
	copy(result[8:], payload)
	return result
}

func TestMonitorReadyAdvertisesCapabilitiesAndUnicodeText(t *testing.T) {
	client := NewClient(&fakeClipboard{})
	sender := &fakeSender{}
	client.Sender(sender)
	client.Process(pdu(cbMonitorReady, 0, nil))
	if len(sender.messages) != 2 {
		t.Fatalf("got %d messages, want 2", len(sender.messages))
	}
	if got := binary.LittleEndian.Uint16(sender.messages[0][:2]); got != cbClipCaps {
		t.Fatalf("first message type = %d", got)
	}
	if got := binary.LittleEndian.Uint16(sender.messages[1][:2]); got != cbFormatList {
		t.Fatalf("second message type = %d", got)
	}
}

func TestLocalUnicodeTextResponse(t *testing.T) {
	clipboard := &fakeClipboard{text: "hello 世界"}
	client := NewClient(clipboard)
	sender := &fakeSender{}
	client.Sender(sender)
	request := make([]byte, 4)
	binary.LittleEndian.PutUint32(request, cfUnicodeText)
	client.Process(pdu(cbFormatDataRequest, 0, request))
	if len(sender.messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(sender.messages))
	}
	response := sender.messages[0]
	if got := binary.LittleEndian.Uint16(response[:2]); got != cbFormatDataResponse {
		t.Fatalf("message type = %d", got)
	}
	if got := decodeUTF16(response[8:]); got != clipboard.text {
		t.Fatalf("decoded text = %q", got)
	}
}

func TestRemoteUnicodeTextWritesClipboard(t *testing.T) {
	clipboard := &fakeClipboard{}
	client := NewClient(clipboard)
	sender := &fakeSender{}
	client.Sender(sender)
	formats := make([]byte, 4)
	binary.LittleEndian.PutUint32(formats, cfUnicodeText)
	client.Process(pdu(cbFormatList, 0, formats))
	client.Process(pdu(cbFormatDataResponse, cbResponseOK, encodeUTF16("remote text")))
	if clipboard.written != "remote text" {
		t.Fatalf("clipboard text = %q", clipboard.written)
	}
}
