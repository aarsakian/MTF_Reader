package dblk

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/aarsakian/MTF_Reader/utils"
)

const DBLK_HDR_LEN = 52
const STREAM_HDR_LEN = 22

// MQDA payload starts 2 bytes after the stream header
const DATA_STREAM_START = 24

type GENERIC_STREAM struct {
	Header *Stream_Header
	Data   bytes.Buffer
}

type DATA_STREAM struct { //mqda
	Header        *Stream_Header
	Data          bytes.Buffer
	AllocatedSize int
}

type PAD_STREAM struct {
	Header *Stream_Header
	Data   []byte //should be null
}

type RAID_STREAM struct {
	Header *Stream_Header
	Data   []byte
}

// 52 bytes
type MTF_DB_HDR struct {
	DBLKType             [4]byte
	BlockAttr            uint32
	FirstEventOffset     uint16 //offset to the first data stream
	OSID                 uint8
	OSVersion            uint8
	DisplayableSize      uint64
	FormatLogicalAddress uint64 //number of Format Logical Blocks from the first MTF_SSET in this Data Set
	MBCReserved          uint16
	Reserved1            [6]byte
	ControlBlockID       uint32
	Reserved2            [4]byte
	OSSpecificDate       utils.MTF_TAPE_ADDRESS
	StringType           uint8
	Reserved3            byte
	Checksum             uint16
}

type MTF_Generic struct {
	CommonBlockHeader *MTF_DB_HDR
}

type MTF_Tape struct { //94 bytes
	CommonBlockHeader           *MTF_DB_HDR
	MediaFamilyID               uint32
	TAPEAttrs                   uint32
	MediaSequenceNumber         uint16
	PasswordEncryptionAlgorithm uint16
	SoftFilemarkBlockSize       uint16
	MediaBasedCatalogType       uint16
	MediaName                   utils.MTF_TAPE_ADDRESS
	MediaDescription            utils.MTF_TAPE_ADDRESS
	MediaPassword               utils.MTF_TAPE_ADDRESS
	SoftwareName                utils.MTF_TAPE_ADDRESS
	FormatLogicalBlockSize      uint16
	SoftwareVendorID            uint16
	MediaDate                   utils.MTF_DATE_TIME
	MTFMajorVersion             uint8
}

type MTF_SSET struct {
	CommonBlockHeader            *MTF_DB_HDR
	SSETAttrs                    uint32
	PasswordEncryptionAlgorithm  uint16
	SoftwareCompressionAlgorithm uint16
	SoftwareVendorID             uint16
	DataSetNumber                uint16
	DataSetName                  utils.MTF_TAPE_ADDRESS
	DataSetDescription           utils.MTF_TAPE_ADDRESS
	DataSetPassword              utils.MTF_TAPE_ADDRESS
	UserName                     utils.MTF_TAPE_ADDRESS
	PhysicalBlockAddress         uint64
	MediaWriteDate               utils.MTF_DATE_TIME
	SoftwareMajorVersion         uint8
	SoftwareMinorVersion         uint8
	TimeZone                     int8
	MTFMinorVersion              uint8
	MediaCatalogVersion          uint8
}

type MTF_SFMB struct {
	CommonBlockHeader   *MTF_DB_HDR
	NumFileMarkEntries  uint32
	FileMarkEntriesUsed uint32
	PBA                 uint32
}

type Stream_Header struct {
	StreamID                 uint32
	StreamFileSystemAttrs    uint16
	StreamMediaFormatAttrs   uint16
	StreamLength             uint64
	DataEncryptionAlgorithm  uint16
	DataCompressionAlgorithm uint16
	Checksum                 uint16
}

type MTF_VOLB struct {
	CommonBlockHeader *MTF_DB_HDR
	VOLBAttr          uint32
	DeviceName        utils.MTF_TAPE_ADDRESS
	VolumeName        utils.MTF_TAPE_ADDRESS
	MachineName       utils.MTF_TAPE_ADDRESS
	MediaWriteDate    utils.MTF_DATE_TIME
}

func (mtf_db_hdr MTF_DB_HDR) GetDBLKTypeStr() string {
	return string(mtf_db_hdr.DBLKType[:])
}

// xor of the 16-bit words preceding the checksum field
func headerChecksum(data []byte, checksumOffset int) uint16 {
	var checksum uint16
	for i := 0; i < checksumOffset; i += 2 {
		checksum ^= binary.LittleEndian.Uint16(data[i:])
	}
	return checksum
}

// IsValidDBLKHeader tells a real descriptor block apart from its type name
// occurring by chance inside other data.
func IsValidDBLKHeader(data []byte) bool {
	if len(data) < DBLK_HDR_LEN {
		return false
	}
	return headerChecksum(data, 50) == binary.LittleEndian.Uint16(data[50:])
}

// IsValidStreamHeader tells a real stream header apart from its id occurring by
// chance inside other data.
func IsValidStreamHeader(data []byte) bool {
	if len(data) < STREAM_HDR_LEN {
		return false
	}
	return headerChecksum(data, 20) == binary.LittleEndian.Uint16(data[20:])
}

func GetStreamLength(data []byte) int64 {
	return int64(binary.LittleEndian.Uint64(data[8:]))
}

func safeUTF16String(data []byte, offset, size uint16) string {
	start := int(offset)
	end := int(offset + size)
	if start < 0 || end > len(data) || start > end {
		return ""
	}
	return utils.DecodeUTF16(data[start:end])
}

func (mtf_tape *MTF_Tape) Parse(data []byte) (int64, error) {
	if len(data) < 94 {
		return 0, errors.New("insufficient data for MTF_Tape header")
	}
	mtf_db_hdr := new(MTF_DB_HDR)
	if err := utils.Unmarshal(data[:52], mtf_db_hdr); err != nil {
		return 0, err
	}
	if err := utils.Unmarshal(data[52:94], mtf_tape); err != nil {
		return 0, err
	}
	mtf_tape.CommonBlockHeader = mtf_db_hdr
	return mtf_tape.getNextOffset(), nil
}

func (mtf_tape *MTF_Tape) GetInfo(data []byte) map[string]string {
	info := map[string]string{}
	info["MediaName"] = safeUTF16String(data, mtf_tape.MediaName.Offset, mtf_tape.MediaName.Size)
	info["MediaDescription"] = safeUTF16String(data, mtf_tape.MediaDescription.Offset, mtf_tape.MediaDescription.Size)
	info["SoftwareName"] = safeUTF16String(data, mtf_tape.SoftwareName.Offset, mtf_tape.SoftwareName.Size)
	info["MediaDate"] = mtf_tape.MediaDate.ToString()
	return info
}

func (mtf_tape MTF_Tape) getNextOffset() int64 {
	return int64(mtf_tape.CommonBlockHeader.FirstEventOffset)
}

func (mtf_sfmb *MTF_SFMB) Parse(data []byte) (int64, error) {
	if len(data) < 64 {
		return 0, errors.New("insufficient data for MTF_SFMB header")
	}
	mtf_db_hdr := new(MTF_DB_HDR)
	if err := utils.Unmarshal(data[:52], mtf_db_hdr); err != nil {
		return 0, err
	}
	if err := utils.Unmarshal(data[52:64], mtf_sfmb); err != nil {
		return 0, err
	}
	mtf_sfmb.CommonBlockHeader = mtf_db_hdr
	return mtf_sfmb.getNextOffset(), nil
}

func (mtf_sfmb MTF_SFMB) getNextOffset() int64 {
	return int64(mtf_sfmb.CommonBlockHeader.FirstEventOffset)
}

func (mtf_sset *MTF_SSET) Parse(data []byte) (int64, error) {
	if len(data) < 98 {
		return 0, errors.New("insufficient data for MTF_SSET header")
	}
	mtf_db_hdr := new(MTF_DB_HDR)
	if err := utils.Unmarshal(data[:52], mtf_db_hdr); err != nil {
		return 0, err
	}
	if err := utils.Unmarshal(data[52:98], mtf_sset); err != nil {
		return 0, err
	}
	mtf_sset.CommonBlockHeader = mtf_db_hdr
	return mtf_sset.getNextOffset(), nil
}

func (mtf_sset MTF_SSET) GetInfo(data []byte) map[string]string {
	info := map[string]string{}
	info["DataSetName"] = safeUTF16String(data, mtf_sset.DataSetName.Offset, mtf_sset.DataSetName.Size)
	info["DataSetDescription"] = safeUTF16String(data, mtf_sset.DataSetDescription.Offset, mtf_sset.DataSetDescription.Size)
	info["DataSetPassword"] = safeUTF16String(data, mtf_sset.DataSetPassword.Offset, mtf_sset.DataSetPassword.Size)
	info["UserName"] = safeUTF16String(data, mtf_sset.UserName.Offset, mtf_sset.UserName.Size)
	info["MediaWriteDate"] = mtf_sset.MediaWriteDate.ToString()
	return info
}

func (mtf_sset MTF_SSET) getNextOffset() int64 {
	return int64(mtf_sset.CommonBlockHeader.FirstEventOffset)
}

func (mtf_volb *MTF_VOLB) Parse(data []byte) (int64, error) {
	if len(data) < 73 {
		return 0, errors.New("insufficient data for MTF_VOLB header")
	}
	mtf_db_hdr := new(MTF_DB_HDR)
	if err := utils.Unmarshal(data[:52], mtf_db_hdr); err != nil {
		return 0, err
	}
	if err := utils.Unmarshal(data[52:73], mtf_volb); err != nil {
		return 0, err
	}
	mtf_volb.CommonBlockHeader = mtf_db_hdr

	return mtf_volb.getNextOffset(), nil
}

func (mtf_volb MTF_VOLB) getNextOffset() int64 {
	return int64(mtf_volb.CommonBlockHeader.FirstEventOffset)
}

func (pad_stream *PAD_STREAM) Parse(data []byte) (int64, error) {
	if len(data) < 22 {
		return 0, errors.New("insufficient data for stream header")
	}
	stream_header := new(Stream_Header)
	if err := utils.Unmarshal(data[:22], stream_header); err != nil {
		return 0, err
	}
	pad_stream.Header = stream_header
	return pad_stream.getNextOffset(), nil
}

func (pad_stream PAD_STREAM) getNextOffset() int64 {
	return int64(pad_stream.Header.StreamLength) + 22
}

func (generic_stream *GENERIC_STREAM) Parse(data []byte) (int64, error) {
	if len(data) < 22 {
		return 0, errors.New("insufficient data for stream header")
	}
	stream_header := new(Stream_Header)
	if err := utils.Unmarshal(data[:22], stream_header); err != nil {
		return 0, err
	}
	generic_stream.Header = stream_header
	generic_stream.Data.Grow(int(stream_header.StreamLength))
	start := 22
	end := int(22 + stream_header.StreamLength)
	if start > len(data) {
		return int64(len(data)), errors.New("insufficient stream payload")
	}
	if end > len(data) {
		generic_stream.Data.Write(data[start:])
		return int64(len(data)), errors.New("exceeded available buffer")
	}
	generic_stream.Data.Write(data[start:end])
	return generic_stream.getNextOffset(), nil
}

func (generic_stream GENERIC_STREAM) getNextOffset() int64 {
	return int64(generic_stream.Header.StreamLength) + 22
}

func (data_stream *DATA_STREAM) Parse(data []byte) (int64, error) {
	if len(data) < 22 {
		return 0, errors.New("insufficient data for stream header")
	}
	stream_header := new(Stream_Header)
	if err := utils.Unmarshal(data[:22], stream_header); err != nil {
		return 0, err
	}
	data_stream.Header = stream_header
	start := DATA_STREAM_START
	end := int(22 + stream_header.StreamLength)
	// size of the payload copied, so that IsFull and AppendData agree with the bytes written here
	data_stream.AllocatedSize = max(end-start, 0)
	data_stream.Data.Grow(data_stream.AllocatedSize)
	if start > len(data) {
		return int64(len(data)), errors.New("insufficient stream payload")
	}
	if end > len(data) {
		data_stream.Data.Write(data[start:])
		return int64(len(data)), errors.New("exceeded available buffer")
	}
	if end > start {
		data_stream.Data.Write(data[start:end])
	}
	return data_stream.getNextOffset(), nil
}

func (data_stream DATA_STREAM) getNextOffset() int64 {
	return int64(data_stream.Header.StreamLength) + 22
}

func (data_stream *DATA_STREAM) AppendData(data []byte) int64 {
	remaining := data_stream.AllocatedSize - data_stream.Data.Len()
	if remaining <= 0 {
		return 0
	}
	if remaining > len(data) {
		data_stream.Data.Write(data)
		return int64(len(data))
	}
	actualBytesWritten := remaining
	if actualBytesWritten > len(data) {
		actualBytesWritten = len(data)
	}
	if actualBytesWritten <= 0 {
		return 0
	}
	data_stream.Data.Write(data[:actualBytesWritten])
	return int64(actualBytesWritten)
}

// Cap() can exceed the requested size after Grow, so compare against the payload size
func (data_stream DATA_STREAM) IsFull() bool {
	return data_stream.Data.Len() >= data_stream.AllocatedSize
}

func (raid_stream *RAID_STREAM) Parse(data []byte) (int64, error) {
	if len(data) < 22 {
		return 0, errors.New("insufficient data for stream header")
	}
	stream_header := new(Stream_Header)
	if err := utils.Unmarshal(data[:22], stream_header); err != nil {
		return 0, err
	}
	raid_stream.Header = stream_header
	streamLen := int(stream_header.StreamLength)
	if len(data) < 22+streamLen {
		return int64(len(data)), errors.New("exceeded available buffer")
	}
	if len(raid_stream.Data) < streamLen {
		raid_stream.Data = make([]byte, streamLen)
	}
	copy(raid_stream.Data, data[22:22+streamLen])
	return raid_stream.getNextOffset(), nil
}

func (raid_stream *RAID_STREAM) getNextOffset() int64 {
	return int64(raid_stream.Header.StreamLength) + 22
}

func (mtf_gen *MTF_Generic) Parse(data []byte) (int64, error) {
	mtf_db_hdr := new(MTF_DB_HDR)
	utils.Unmarshal(data[:52], mtf_db_hdr)
	mtf_gen.CommonBlockHeader = mtf_db_hdr
	return mtf_gen.getNextOffset(), nil
}

func (mtf_gen MTF_Generic) getNextOffset() int64 {
	return int64(mtf_gen.CommonBlockHeader.FirstEventOffset)
}
