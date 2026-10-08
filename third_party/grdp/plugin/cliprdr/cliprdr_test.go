package cliprdr

import (
	"bytes"
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
	flags := binary.LittleEndian.Uint32(sender.messages[0][20:24])
	if flags&cbStreamFileClipEnabled == 0 {
		t.Fatal("file clipboard capability was not advertised")
	}
}

func TestShareFileDescriptorAndContents(t *testing.T) {
	contents := []byte("0123456789")
	client := NewClient(&fakeClipboard{})
	sender := &fakeSender{}
	client.Sender(sender)
	if err := client.ShareFile(SharedFile{Name: "report.txt", Size: int64(len(contents)), Reader: bytes.NewReader(contents)}); err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint16(sender.messages[0][:2]); got != cbFormatList {
		t.Fatalf("message type = %d", got)
	}

	descriptorRequest := make([]byte, 4)
	binary.LittleEndian.PutUint32(descriptorRequest, cfFileDescriptorW)
	client.Process(pdu(cbFormatDataRequest, 0, descriptorRequest))
	descriptor := sender.messages[1][8:]
	if got := binary.LittleEndian.Uint32(descriptor[:4]); got != 1 {
		t.Fatalf("file count = %d", got)
	}
	if got := binary.LittleEndian.Uint32(descriptor[72:76]); got != uint32(len(contents)) {
		t.Fatalf("file size = %d", got)
	}
	if got := decodeUTF16(descriptor[76:]); got != "report.txt" {
		t.Fatalf("file name = %q", got)
	}

	request := make([]byte, 24)
	binary.LittleEndian.PutUint32(request[0:4], 42)
	binary.LittleEndian.PutUint32(request[8:12], fileContentsRange)
	binary.LittleEndian.PutUint64(request[12:20], 2)
	binary.LittleEndian.PutUint32(request[20:24], 4)
	client.Process(pdu(cbFileContentsRequest, 0, request))
	response := sender.messages[2]
	if got := binary.LittleEndian.Uint16(response[:2]); got != cbFileContentsResponse {
		t.Fatalf("message type = %d", got)
	}
	if got := binary.LittleEndian.Uint32(response[8:12]); got != 42 {
		t.Fatalf("stream id = %d", got)
	}
	if got := string(response[12:]); got != "2345" {
		t.Fatalf("file contents = %q", got)
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
