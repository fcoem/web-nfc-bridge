//go:build darwin

package bridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/ebfe/scard"
)

const (
	type2CapabilityPage = 3
	type2UserDataPage   = 4
	type2PageSize       = 4
	type2ReadChunkBytes = 0x10
)

type type2Capability struct {
	DataAreaBytes int
	ReadOnly      bool
}

type type2WriteResponseError struct {
	Page     int
	Response []byte
}

func (e *type2WriteResponseError) Error() string {
	return describeType2WriteResponse(e.Page, e.Response)
}

func buildNDEFMessage(request *WriteRequest) []byte {
	typeBytes := []byte(request.MediaType)
	payloadBytes := request.EncodedPayload

	flags := byte(0x80 | 0x40 | 0x02)
	message := make([]byte, 0, len(payloadBytes)+len(typeBytes)+8)

	if len(payloadBytes) <= 0xFF {
		flags |= 0x10
		message = append(message, flags, byte(len(typeBytes)), byte(len(payloadBytes)))
	} else {
		message = append(
			message,
			flags,
			byte(len(typeBytes)),
			byte(len(payloadBytes)>>24),
			byte(len(payloadBytes)>>16),
			byte(len(payloadBytes)>>8),
			byte(len(payloadBytes)),
		)
	}

	message = append(message, typeBytes...)
	message = append(message, payloadBytes...)
	return message
}

func buildType2TLV(message []byte) []byte {
	tl := make([]byte, 0, len(message)+4)
	tl = append(tl, 0x03)
	if len(message) <= 0xFE {
		tl = append(tl, byte(len(message)))
	} else {
		tl = append(tl, 0xFF, byte(len(message)>>8), byte(len(message)))
	}
	tl = append(tl, message...)
	tl = append(tl, 0xFE)
	if rem := len(tl) % type2PageSize; rem != 0 {
		tl = append(tl, bytes.Repeat([]byte{0x00}, type2PageSize-rem)...)
	}
	return tl
}

func readType2Capability(card *scard.Card) (*type2Capability, error) {
	data, err := readType2PagesWithFallback(card, type2CapabilityPage, type2ReadChunkBytes)
	if err != nil {
		return nil, err
	}
	if len(data) < 4 {
		return nil, errors.New("invalid capability container length")
	}
	if data[0] != 0xE1 {
		return nil, errors.New("card is not NDEF formatted")
	}
	return &type2Capability{
		DataAreaBytes: int(data[2]) * 8,
		ReadOnly:      data[3]&0x0F != 0x00,
	}, nil
}

func readType2Pages(card *scard.Card, startPage int, length int) ([]byte, error) {
	command := []byte{0xFF, 0xB0, 0x00, byte(startPage), byte(length)}
	response, err := card.Transmit(command)
	if err != nil {
		return nil, err
	}
	if len(response) < 2 || response[len(response)-2] != 0x90 || response[len(response)-1] != 0x00 {
		return nil, fmt.Errorf("unexpected read response: % X", response)
	}
	return response[:len(response)-2], nil
}

func writeType2Pages(card *scard.Card, startPage int, payload []byte) error {
	for offset := 0; offset < len(payload); offset += type2PageSize {
		chunk := payload[offset : offset+type2PageSize]
		page := startPage + offset/type2PageSize
		command := append([]byte{0xFF, 0xD6, 0x00, byte(page), byte(type2PageSize)}, chunk...)
		response, err := card.Transmit(command)
		if err != nil {
			return err
		}
		if len(response) != 2 || response[0] != 0x90 || response[1] != 0x00 {
			return &type2WriteResponseError{Page: page, Response: append([]byte(nil), response...)}
		}
	}
	return nil
}

func describeType2WriteResponse(page int, response []byte) string {
	if len(response) == 2 && response[0] == 0x63 && response[1] == 0x00 {
		if page == type2UserDataPage {
			return fmt.Sprintf("card rejected write at first data page %d: % X; tag may be locked, read-only, or require reformatting", page, response)
		}
		return fmt.Sprintf("card rejected write at page %d: % X; tag may be locked or no longer writable", page, response)
	}

	return fmt.Sprintf("unexpected write response at page %d: % X", page, response)
}

func verifyType2Write(card *scard.Card, expected []byte) error {
	readBack, err := readType2Range(card, type2UserDataPage, len(expected))
	if err != nil {
		return err
	}
	if !bytes.Equal(readBack, expected) {
		return errors.New("written NDEF payload did not verify")
	}
	return nil
}

func readType2Range(card *scard.Card, startPage int, totalBytes int) ([]byte, error) {
	result := make([]byte, 0, totalBytes)
	remaining := totalBytes
	page := startPage
	for remaining > 0 {
		chunkLength := remaining
		if chunkLength > type2ReadChunkBytes {
			chunkLength = type2ReadChunkBytes
		}
		chunk, err := readType2PagesWithFallback(card, page, chunkLength)
		if err != nil {
			return nil, err
		}
		result = append(result, chunk...)
		remaining -= chunkLength
		page += chunkLength / type2PageSize
	}
	return result, nil
}

func readType2PagesWithFallback(card *scard.Card, startPage int, length int) ([]byte, error) {
	data, err := readType2Pages(card, startPage, length)
	if err == nil {
		return data, nil
	}
	if !isType2ReadLengthError(err) || length <= type2PageSize {
		return nil, err
	}

	result := make([]byte, 0, length)
	remaining := length
	page := startPage
	for remaining > 0 {
		chunkLength := remaining
		if chunkLength > type2PageSize {
			chunkLength = type2PageSize
		}
		chunk, chunkErr := readType2Pages(card, page, chunkLength)
		if chunkErr != nil {
			return nil, chunkErr
		}
		result = append(result, chunk...)
		remaining -= chunkLength
		page += chunkLength / type2PageSize
	}

	return result, nil
}

func isType2ReadLengthError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unexpected read response: 63 00")
}

func requiredType2Pages(length int) int {
	return int(math.Ceil(float64(length) / float64(type2PageSize)))
}

func readType2NDEF(card *scard.Card, capability *type2Capability) (string, map[string]any, error) {
	data, err := readType2Range(card, type2UserDataPage, capability.DataAreaBytes)
	if err != nil {
		return "", nil, err
	}

	mediaType, payloadBytes, err := parseType2NDEFPayload(data)
	if err != nil {
		return "", nil, err
	}
	if mediaType == "" || len(payloadBytes) == 0 {
		return "", nil, nil
	}

	if mediaType != NDEFApplicationJSON {
		return mediaType, map[string]any{
			"raw": string(payloadBytes),
		}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return mediaType, map[string]any{
			"raw": string(payloadBytes),
		}, nil
	}

	return mediaType, payload, nil
}

func parseType2NDEFPayload(data []byte) (string, []byte, error) {
	if len(data) < 3 {
		return "", nil, nil
	}

	if data[0] != 0x03 {
		return "", nil, nil
	}

	index := 1
	length := 0
	if data[index] == 0xFF {
		if len(data) < 4 {
			return "", nil, errors.New("invalid extended NDEF TLV")
		}
		length = int(data[index+1])<<8 | int(data[index+2])
		index += 3
	} else {
		length = int(data[index])
		index++
	}

	if length == 0 || len(data) < index+length {
		return "", nil, nil
	}

	message := data[index : index+length]
	return parseNDEFMessage(message)
}

func parseNDEFMessage(message []byte) (string, []byte, error) {
	if len(message) < 3 {
		return "", nil, errors.New("invalid NDEF message")
	}

	flags := message[0]
	tnf := flags & 0x07
	shortRecord := flags&0x10 != 0
	idLengthPresent := flags&0x08 != 0

	if tnf != 0x02 {
		return "", nil, nil
	}

	index := 1
	typeLength := int(message[index])
	index++

	payloadLength := 0
	if shortRecord {
		if len(message) <= index {
			return "", nil, errors.New("invalid short NDEF message")
		}
		payloadLength = int(message[index])
		index++
	} else {
		if len(message) < index+4 {
			return "", nil, errors.New("invalid NDEF message length")
		}
		payloadLength = int(message[index])<<24 | int(message[index+1])<<16 | int(message[index+2])<<8 | int(message[index+3])
		index += 4
	}

	if idLengthPresent {
		if len(message) <= index {
			return "", nil, errors.New("invalid NDEF id length")
		}
		idLength := int(message[index])
		index++
		if len(message) < index+idLength {
			return "", nil, errors.New("invalid NDEF id")
		}
		index += idLength
	}

	if len(message) < index+typeLength+payloadLength {
		return "", nil, errors.New("invalid NDEF payload bounds")
	}

	mediaType := string(message[index : index+typeLength])
	index += typeLength
	payload := message[index : index+payloadLength]
	return mediaType, payload, nil
}