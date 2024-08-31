package dblk

import (
	"bytes"
	"errors"

	"github.com/aarsakian/MTF_Reader/utils"
)

type GENERIC_STREAM struct {
	Header *Stream_Header
	Data   bytes.Buffer
}

type DATA_STREAM struct {
	Header *Stream_Header
	Data   bytes.Buffer
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

func (mtf_tape *MTF_Tape) Parse(data []byte) (int64, error) {
	mtf_db_hdr := new(MTF_DB_HDR)
	utils.Unmarshal(data[:52], mtf_db_hdr)
	utils.Unmarshal(data[52:94], mtf_tape)
	mtf_tape.CommonBlockHeader = mtf_db_hdr
	return mtf_tape.getNextOffset(), nil

}

func (mtf_tape MTF_Tape) getNextOffset() int64 {
	return int64(mtf_tape.CommonBlockHeader.FirstEventOffset)
}

func (mtf_sfmb *MTF_SFMB) Parse(data []byte) (int64, error) {
	mtf_db_hdr := new(MTF_DB_HDR)
	utils.Unmarshal(data[:52], mtf_db_hdr)

	utils.Unmarshal(data[52:64], mtf_sfmb)
	mtf_sfmb.CommonBlockHeader = mtf_db_hdr
	return mtf_sfmb.getNextOffset(), nil
}

func (mtf_sfmb MTF_SFMB) getNextOffset() int64 {
	return int64(mtf_sfmb.CommonBlockHeader.FirstEventOffset)
}

func (mtf_sset *MTF_SSET) Parse(data []byte) (int64, error) {
	mtf_db_hdr := new(MTF_DB_HDR)
	utils.Unmarshal(data[:52], mtf_db_hdr)

	utils.Unmarshal(data[52:98], mtf_sset)
	mtf_sset.CommonBlockHeader = mtf_db_hdr
	return mtf_sset.getNextOffset(), nil
}

func (mtf_sset MTF_SSET) getNextOffset() int64 {
	return int64(mtf_sset.CommonBlockHeader.FirstEventOffset)
}

func (mtf_volb *MTF_VOLB) Parse(data []byte) (int64, error) {
	mtf_db_hdr := new(MTF_DB_HDR)
	utils.Unmarshal(data[:52], mtf_db_hdr)

	utils.Unmarshal(data[52:73], mtf_volb)
	mtf_volb.CommonBlockHeader = mtf_db_hdr

	return mtf_volb.getNextOffset(), nil
}

func (mtf_volb MTF_VOLB) getNextOffset() int64 {
	return int64(mtf_volb.CommonBlockHeader.FirstEventOffset)
}

func (pad_stream *PAD_STREAM) Parse(data []byte) (int64, error) {
	stream_header := new(Stream_Header)
	utils.Unmarshal(data[:22], stream_header)
	pad_stream.Header = stream_header

	return pad_stream.getNextOffset(), nil

}

func (pad_stream PAD_STREAM) getNextOffset() int64 {
	return int64(pad_stream.Header.StreamLength) + 22
}

func (generic_stream *GENERIC_STREAM) Parse(data []byte) (int64, error) {
	stream_header := new(Stream_Header)
	utils.Unmarshal(data[:22], stream_header)
	generic_stream.Header = stream_header
	generic_stream.Data.Grow(int(stream_header.StreamLength))
	if len(data) >= int(22+stream_header.StreamLength) {
		generic_stream.Data.Write(data[22 : 22+stream_header.StreamLength])
		return generic_stream.getNextOffset(), nil
	} else {
		generic_stream.Data.Write(data[22:])
		return int64(len(data)), errors.New("exceeded available buffer")
	}

}

func (generic_stream GENERIC_STREAM) getNextOffset() int64 {
	return int64(generic_stream.Header.StreamLength) + 22
}

func (data_stream *DATA_STREAM) Parse(data []byte) (int64, error) {
	stream_header := new(Stream_Header)
	utils.Unmarshal(data[:22], stream_header)
	data_stream.Header = stream_header
	data_stream.Data.Grow(int(stream_header.StreamLength))
	if len(data) >= int(22+stream_header.StreamLength) {
		data_stream.Data.Write(data[22 : 22+stream_header.StreamLength])
		return data_stream.getNextOffset(), nil
	} else {
		data_stream.Data.Write(data[22:])
		return int64(len(data)), errors.New("exceeded available buffer")
	}

}

func (data_stream DATA_STREAM) getNextOffset() int64 {
	return int64(data_stream.Header.StreamLength) + 22
}

func (data_stream *DATA_STREAM) AppendData(data []byte) {
	data_stream.Data.Write(data)
}

func (raid_stream *RAID_STREAM) Parse(data []byte) (int64, error) {
	stream_header := new(Stream_Header)
	utils.Unmarshal(data[:22], stream_header)
	raid_stream.Header = stream_header
	if len(data) >= int(22+stream_header.StreamLength) {
		copy(raid_stream.Data, data[22:22+stream_header.StreamLength])
		return raid_stream.getNextOffset(), nil
	} else {

		return int64(len(data)), errors.New("exceeded available buffer")
	}
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
