package cliprdr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"
	"unicode/utf16"

	"github.com/nakagami/grdp/core"
	"github.com/nakagami/grdp/plugin"
)

const (
	ChannelName           = plugin.CLIPRDR_SVC_CHANNEL_NAME
	ChannelOption         = plugin.CHANNEL_OPTION_INITIALIZED | plugin.CHANNEL_OPTION_ENCRYPT_RDP | plugin.CHANNEL_OPTION_COMPRESS_RDP | plugin.CHANNEL_OPTION_SHOW_PROTOCOL
	cbMonitorReady        = 1
	cbFormatList          = 2
	cbFormatListResponse  = 3
	cbFormatDataRequest   = 4
	cbFormatDataResponse  = 5
	cbClipCaps            = 7
	cbResponseOK          = 1
	cbResponseFail        = 2
	cbCapsTypeGeneral     = 1
	cbCapsVersion2        = 2
	cbUseLongFormatNames  = 2
	cfUnicodeText         = 13
	maxClipboardTextBytes = 1024 * 1024
)

type Clipboard interface {
	ReadText() (string, error)
	WriteText(string) error
}

// Client implements the text-only subset of MS-RDPECLIP.
type Client struct {
	sender             core.ChannelSender
	clipboard          Clipboard
	useLongFormatNames bool
	waitingForText     bool
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
	}
}

func (c *Client) AnnounceText() {
	c.stateMu.Lock()
	useLongFormatNames := c.useLongFormatNames
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
	if len(payload) < 4 || binary.LittleEndian.Uint32(payload[:4]) != cfUnicodeText {
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
	binary.LittleEndian.PutUint32(payload[12:16], cbUseLongFormatNames)
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
