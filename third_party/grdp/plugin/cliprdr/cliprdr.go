package cliprdr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"unicode/utf16"

	"github.com/nakagami/grdp/core"
	"github.com/nakagami/grdp/plugin"
)

const (
	ChannelName             = plugin.CLIPRDR_SVC_CHANNEL_NAME
	ChannelOption           = plugin.CHANNEL_OPTION_INITIALIZED | plugin.CHANNEL_OPTION_ENCRYPT_RDP | plugin.CHANNEL_OPTION_COMPRESS_RDP | plugin.CHANNEL_OPTION_SHOW_PROTOCOL
	cbMonitorReady          = 1
	cbFormatList            = 2
	cbFormatListResponse    = 3
	cbFormatDataRequest     = 4
	cbFormatDataResponse    = 5
	cbClipCaps              = 7
	cbFileContentsRequest   = 8
	cbFileContentsResponse  = 9
	cbResponseOK            = 1
	cbResponseFail          = 2
	cbCapsTypeGeneral       = 1
	cbCapsVersion2          = 2
	cbUseLongFormatNames    = 2
	cbStreamFileClipEnabled = 4
	cbFileClipNoFilePaths   = 8
	cfUnicodeText           = 13
	cfFileDescriptorW       = 0xC001
	cfFileContents          = 0xC002
	fileContentsSize        = 1
	fileContentsRange       = 2
	fileAttributeNormal     = 0x80
	fdAttributes            = 0x00000004
	fdFileSize              = 0x00000040
	fdProgressUI            = 0x00004000
	maxClipboardTextBytes   = 1024 * 1024
	maxFileChunkBytes       = 1024 * 1024
)

type Clipboard interface {
	ReadText() (string, error)
	WriteText(string) error
}

type SharedFile struct {
	Name   string
	Size   int64
	Reader io.ReaderAt
}

// Client implements the text-only subset of MS-RDPECLIP.
type Client struct {
	sender             core.ChannelSender
	clipboard          Clipboard
	useLongFormatNames bool
	waitingForText     bool
	sharedFile         *SharedFile
	stateMu            sync.Mutex
	sendMu             sync.Mutex
}

func NewClient(clipboard Clipboard) *Client        { return &Client{clipboard: clipboard} }
func (c *Client) GetType() (string, uint32)        { return ChannelName, ChannelOption }
func (c *Client) Sender(sender core.ChannelSender) { c.sender = sender }
func (c *Client) Send(data []byte) (int, error) {
	if c.sender == nil {
		return 0, fmt.Errorf("cliprdr sender is not connected")
	}
	return c.sender.SendToChannel(ChannelName, data)
}

func (c *Client) Process(data []byte) {
	if len(data) < 8 {
		return
	}
	msgType := binary.LittleEndian.Uint16(data[0:2])
	flags := binary.LittleEndian.Uint16(data[2:4])
	length := binary.LittleEndian.Uint32(data[4:8])
	if length > maxClipboardTextBytes || int(length) > len(data)-8 {
		slog.Warn("cliprdr: invalid PDU length", "length", length)
		return
	}
	payload := data[8 : 8+length]
	switch msgType {
	case cbClipCaps:
		c.processCapabilities(payload)
	case cbMonitorReady:
		c.sendCapabilities()
		c.AnnounceText()
	case cbFormatList:
		c.sendHeader(cbFormatListResponse, cbResponseOK, nil)
		c.processFormatList(payload)
	case cbFormatDataRequest:
		c.processDataRequest(payload)
	case cbFormatDataResponse:
		c.processDataResponse(flags, payload)
	case cbFileContentsRequest:
		c.processFileContentsRequest(payload)
	}
}

func (c *Client) AnnounceText() {
	c.stateMu.Lock()
	useLongFormatNames := c.useLongFormatNames
	c.sharedFile = nil
	c.stateMu.Unlock()
	payload := &bytes.Buffer{}
	_ = binary.Write(payload, binary.LittleEndian, uint32(cfUnicodeText))
	if useLongFormatNames {
		_ = binary.Write(payload, binary.LittleEndian, uint16(0))
	} else {
		payload.Write(make([]byte, 32))
	}
	c.sendHeader(cbFormatList, 0, payload.Bytes())
}

func (c *Client) ShareFile(file SharedFile) error {
	if file.Name == "" || file.Size < 0 || file.Reader == nil {
		return fmt.Errorf("invalid shared file")
	}
	c.stateMu.Lock()
	c.sharedFile = &file
	useLongFormatNames := c.useLongFormatNames
	c.stateMu.Unlock()
	payload := &bytes.Buffer{}
	writeFormat(payload, cfFileDescriptorW, "FileGroupDescriptorW", useLongFormatNames)
	writeFormat(payload, cfFileContents, "FileContents", useLongFormatNames)
	c.sendHeader(cbFormatList, 0, payload.Bytes())
	return nil
}

func writeFormat(payload *bytes.Buffer, id uint32, name string, longNames bool) {
	_ = binary.Write(payload, binary.LittleEndian, id)
	if longNames {
		payload.Write(encodeUTF16(name))
		return
	}
	fixedName := make([]byte, 32)
	copy(fixedName, []byte(name))
	payload.Write(fixedName)
}

func (c *Client) processCapabilities(payload []byte) {
	if len(payload) >= 16 {
		c.stateMu.Lock()
		c.useLongFormatNames = binary.LittleEndian.Uint32(payload[12:16])&cbUseLongFormatNames != 0
		c.stateMu.Unlock()
	}
}

func (c *Client) processFormatList(payload []byte) {
	c.stateMu.Lock()
	useLongFormatNames := c.useLongFormatNames
	c.stateMu.Unlock()
	for offset := 0; offset+4 <= len(payload); {
		formatID := binary.LittleEndian.Uint32(payload[offset : offset+4])
		offset += 4
		if formatID == cfUnicodeText {
			c.waitingForText = true
			request := make([]byte, 4)
			binary.LittleEndian.PutUint32(request, cfUnicodeText)
			c.sendHeader(cbFormatDataRequest, 0, request)
			return
		}
		if useLongFormatNames {
			for offset+1 < len(payload) {
				value := binary.LittleEndian.Uint16(payload[offset : offset+2])
				offset += 2
				if value == 0 {
					break
				}
			}
		} else {
			offset += 32
		}
	}
}

func (c *Client) processDataRequest(payload []byte) {
	if len(payload) < 4 {
		c.sendHeader(cbFormatDataResponse, cbResponseFail, nil)
		return
	}
	formatID := binary.LittleEndian.Uint32(payload[:4])
	if formatID == cfFileDescriptorW {
		c.sendFileDescriptor()
		return
	}
	if formatID != cfUnicodeText {
		c.sendHeader(cbFormatDataResponse, cbResponseFail, nil)
		return
	}
	text, err := c.clipboard.ReadText()
	if err != nil {
		c.sendHeader(cbFormatDataResponse, cbResponseFail, nil)
		return
	}
	encoded := encodeUTF16(text)
	if len(encoded) > maxClipboardTextBytes {
		c.sendHeader(cbFormatDataResponse, cbResponseFail, nil)
		return
	}
	c.sendHeader(cbFormatDataResponse, cbResponseOK, encoded)
}

func (c *Client) sendFileDescriptor() {
	c.stateMu.Lock()
	file := c.sharedFile
	c.stateMu.Unlock()
	if file == nil {
		c.sendHeader(cbFormatDataResponse, cbResponseFail, nil)
		return
	}
	descriptor := make([]byte, 4+592)
	binary.LittleEndian.PutUint32(descriptor[0:4], 1)
	base := 4
	binary.LittleEndian.PutUint32(descriptor[base:base+4], fdAttributes|fdFileSize|fdProgressUI)
	binary.LittleEndian.PutUint32(descriptor[base+36:base+40], fileAttributeNormal)
	binary.LittleEndian.PutUint32(descriptor[base+64:base+68], uint32(uint64(file.Size)>>32))
	binary.LittleEndian.PutUint32(descriptor[base+68:base+72], uint32(file.Size))
	nameUnits := make([]uint16, 0, 259)
	for _, value := range file.Name {
		encodedRune := utf16.Encode([]rune{value})
		if len(nameUnits)+len(encodedRune) > 259 {
			break
		}
		nameUnits = append(nameUnits, encodedRune...)
	}
	name := make([]byte, len(nameUnits)*2)
	for i, unit := range nameUnits {
		binary.LittleEndian.PutUint16(name[i*2:], unit)
	}
	copy(descriptor[base+72:base+592], name)
	c.sendHeader(cbFormatDataResponse, cbResponseOK, descriptor)
}

func (c *Client) processFileContentsRequest(payload []byte) {
	if len(payload) < 24 {
		c.sendFileContentsResponse(0, cbResponseFail, nil)
		return
	}
	streamID := binary.LittleEndian.Uint32(payload[0:4])
	listIndex := binary.LittleEndian.Uint32(payload[4:8])
	flags := binary.LittleEndian.Uint32(payload[8:12])
	offset := int64(binary.LittleEndian.Uint64(payload[12:20]))
	requested := binary.LittleEndian.Uint32(payload[20:24])
	c.stateMu.Lock()
	file := c.sharedFile
	c.stateMu.Unlock()
	if file == nil || listIndex != 0 || offset < 0 {
		c.sendFileContentsResponse(streamID, cbResponseFail, nil)
		return
	}
	if flags&fileContentsSize != 0 {
		size := make([]byte, 8)
		binary.LittleEndian.PutUint64(size, uint64(file.Size))
		c.sendFileContentsResponse(streamID, cbResponseOK, size)
		return
	}
	if flags&fileContentsRange == 0 || requested > maxFileChunkBytes || offset > file.Size {
		c.sendFileContentsResponse(streamID, cbResponseFail, nil)
		return
	}
	remaining := file.Size - offset
	if int64(requested) > remaining {
		requested = uint32(remaining)
	}
	data := make([]byte, requested)
	n, err := file.Reader.ReadAt(data, offset)
	if err != nil && err != io.EOF {
		c.sendFileContentsResponse(streamID, cbResponseFail, nil)
		return
	}
	c.sendFileContentsResponse(streamID, cbResponseOK, data[:n])
}

func (c *Client) sendFileContentsResponse(streamID uint32, flags uint16, data []byte) {
	payload := make([]byte, 4+len(data))
	binary.LittleEndian.PutUint32(payload[:4], streamID)
	copy(payload[4:], data)
	c.sendHeader(cbFileContentsResponse, flags, payload)
}

func (c *Client) processDataResponse(flags uint16, payload []byte) {
	if !c.waitingForText {
		return
	}
	c.waitingForText = false
	if flags == cbResponseOK {
		if err := c.clipboard.WriteText(decodeUTF16(payload)); err != nil {
			slog.Warn("cliprdr: clipboard write failed", "error", err)
		}
	}
}

func (c *Client) sendCapabilities() {
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint16(payload[0:2], 1)
	binary.LittleEndian.PutUint16(payload[4:6], cbCapsTypeGeneral)
	binary.LittleEndian.PutUint16(payload[6:8], 12)
	binary.LittleEndian.PutUint32(payload[8:12], cbCapsVersion2)
	binary.LittleEndian.PutUint32(payload[12:16], cbUseLongFormatNames|cbStreamFileClipEnabled|cbFileClipNoFilePaths)
	c.sendHeader(cbClipCaps, 0, payload)
}

func (c *Client) sendHeader(msgType, flags uint16, payload []byte) {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	pdu := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint16(pdu[0:2], msgType)
	binary.LittleEndian.PutUint16(pdu[2:4], flags)
	binary.LittleEndian.PutUint32(pdu[4:8], uint32(len(payload)))
	copy(pdu[8:], payload)
	if _, err := c.Send(pdu); err != nil {
		slog.Warn("cliprdr: send failed", "error", err)
	}
}

func encodeUTF16(value string) []byte {
	units := append(utf16.Encode([]rune(value)), 0)
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[i*2:], unit)
	}
	return data
}

func decodeUTF16(data []byte) string {
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		unit := binary.LittleEndian.Uint16(data[i : i+2])
		if unit == 0 {
			break
		}
		units = append(units, unit)
	}
	return string(utf16.Decode(units))
}
