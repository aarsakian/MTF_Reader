package mtf

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aarsakian/MTF_Reader/dblk"
	"github.com/aarsakian/MTF_Reader/logger"
)

var BUF_SIZE int64 = 100000 * 1024

// bytes needed to validate a header before parsing it
var headerLens = map[string]int64{
	"TAPE": dblk.DBLK_HDR_LEN, "SFMB": dblk.DBLK_HDR_LEN, "SSET": dblk.DBLK_HDR_LEN,
	"VOLB": dblk.DBLK_HDR_LEN, "MSCI": dblk.DBLK_HDR_LEN, "MSDA": dblk.DBLK_HDR_LEN,
	"SPAD": dblk.DATA_STREAM_START, "RAID": dblk.DATA_STREAM_START, "MQCI": dblk.DATA_STREAM_START,
	"APAD": dblk.DATA_STREAM_START, "CSUM": dblk.DATA_STREAM_START, "MQDA": dblk.DATA_STREAM_START,
}

type MTF struct {
	MediaHeader *Media_Header
	DataSet     *DataSet
	Fname       string
}

type Media_Header struct {
	Tape     *dblk.MTF_Tape
	Info     map[string]string
	FileMark *dblk.MTF_SFMB
}

type DataSet struct {
	MTF_SSET        *dblk.MTF_SSET
	Generic_streams []*dblk.GENERIC_STREAM
	Data_stream     *dblk.DATA_STREAM
	Pad_stream      *dblk.PAD_STREAM
	MTF_VOLB        *dblk.MTF_VOLB
	Info            map[string]string
}

func (mtf MTF) ShowInfo() {
	mtf.MediaHeader.showInfo()
	mtf.DataSet.showInfo()
}

func (mtf MTF) Export(exportPath string) int {
	return mtf.DataSet.Export(exportPath)
}

func (mtf *MTF) Process() {
	var buffer []byte

	fhadler, err := os.Open(mtf.Fname)
	if err != nil {

		log.Fatal(err)
	}
	defer fhadler.Close()

	offset := int64(0)

	fsize, err := fhadler.Stat()
	if err != nil {
		logger.MTFlogger.Error(err)

	}
	if fsize.Size() < BUF_SIZE {
		buffer = make([]byte, fsize.Size())
	} else {
		buffer = make([]byte, BUF_SIZE)
	}

	media_header := new(Media_Header)
	data_set := new(DataSet)

	latest_attribute := ""
	for offset < fsize.Size() {
		n, err := fhadler.ReadAt(buffer, offset)
		if err != nil && err != io.EOF {
			logger.MTFlogger.Error(err)
			break
		}
		if n == 0 {
			break
		}
		chunk := buffer[:n] // the last read is shorter than the buffer
		lastChunk := offset+int64(n) >= fsize.Size()

		innerOffset := int64(0)
		for innerOffset < int64(len(chunk)) {
			if innerOffset+4 > int64(len(chunk)) {
				break
			}
			pos := innerOffset
			header := string(chunk[innerOffset : innerOffset+4])
			if headerLen, ok := headerLens[header]; ok {
				if innerOffset+headerLen > int64(len(chunk)) {
					if !lastChunk {
						break // header spans the read boundary, re-read starting from it
					}
					header = ""
				} else if headerLen == dblk.DBLK_HDR_LEN && !dblk.IsValidDBLKHeader(chunk[innerOffset:]) ||
					headerLen != dblk.DBLK_HDR_LEN && !dblk.IsValidStreamHeader(chunk[innerOffset:]) {
					header = "" // block name occurring inside other data
				}
			}

			if header == "TAPE" {

				mtf_tape := new(dblk.MTF_Tape)
				next_offset, err := mtf_tape.Parse(chunk[innerOffset:])

				media_header.Info = mtf_tape.GetInfo(chunk[innerOffset:])
				media_header.Tape = mtf_tape
				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if header == "SFMB" {

				mtf_sfmb := new(dblk.MTF_SFMB)
				next_offset, err := mtf_sfmb.Parse(chunk[innerOffset:])

				media_header.FileMark = mtf_sfmb
				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if header == "SSET" {

				mtf_sset := new(dblk.MTF_SSET)
				next_offset, err := mtf_sset.Parse(chunk[innerOffset:])

				data_set.Info = mtf_sset.GetInfo(chunk[innerOffset:])
				data_set.MTF_SSET = mtf_sset
				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if header == "VOLB" {

				mtf_volb := new(dblk.MTF_VOLB)
				next_offset, err := mtf_volb.Parse(chunk[innerOffset:])

				data_set.MTF_VOLB = mtf_volb
				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if header == "SPAD" {
				pad_stream := new(dblk.PAD_STREAM)
				next_offset, err := pad_stream.Parse(chunk[innerOffset:])

				data_set.Pad_stream = pad_stream
				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if header == "RAID" {
				raid_stream := new(dblk.RAID_STREAM)
				next_offset, err := raid_stream.Parse(chunk[innerOffset:])

				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if header == "MSCI" || header == "MSDA" {
				mtf_generic := new(dblk.MTF_Generic)
				next_offset, err := mtf_generic.Parse(chunk[innerOffset:])

				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if header == "MQCI" || header == "APAD" || header == "CSUM" {
				generic_stream := new(dblk.GENERIC_STREAM)
				next_offset, err := generic_stream.Parse(chunk[innerOffset:])

				data_set.Generic_streams = append(data_set.Generic_streams, generic_stream)
				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if header == "MQDA" && data_set.Data_stream != nil && data_set.Data_stream.AllocatedSize > 0 {
				// the first non-empty MQDA holds the database pages, later ones are
				// re-copies of pages written at the end of the backup, skip them
				innerOffset += dblk.GetStreamLength(chunk[innerOffset:]) + dblk.STREAM_HDR_LEN
				continue

			} else if header == "MQDA" {
				data_stream := new(dblk.DATA_STREAM)
				next_offset, err := data_stream.Parse(chunk[innerOffset:])

				data_set.Data_stream = data_stream
				innerOffset += next_offset
				if err != nil {
					latest_attribute = "MQDA"
					break
				}
			} else if latest_attribute == "MQDA" && !data_set.IsFull() {
				remaining := int64(len(chunk)) - innerOffset
				if remaining <= 0 {
					break
				}
				written := data_set.AppendData(chunk[innerOffset:])
				innerOffset += written

			} else {
				innerOffset += 1 //brute force search alignment??
				logger.MTFlogger.Warning(fmt.Sprintf("Searching for signatures %d", offset))
			}
			if innerOffset <= pos { // block without a forward offset, keep scanning
				innerOffset = pos + 1
			}
		}
		if innerOffset == 0 { // truncated block at the end of the file
			innerOffset = 1
		}
		offset += innerOffset

	}
	mtf.MediaHeader = media_header
	mtf.DataSet = data_set
}

func (dataset *DataSet) AppendData(data []byte) int64 {
	return dataset.Data_stream.AppendData(data)
}

func (dataset DataSet) IsFull() bool {
	return dataset.Data_stream.IsFull()
}

func (mtf MTF) GetExportFileName() string {
	return mtf.DataSet.GetExportName()
}

func (dataset DataSet) GetExportName() string {
	return strings.Replace(dataset.Info["DataSetName"], " ", "_", -1) + ".mdf"
}

func (dataset DataSet) Export(exportPath string) int {
	var err error

	err = os.Mkdir(exportPath, 0750)
	if err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}
	var fhandler *os.File
	nofBytesWritten := 0
	exportName := dataset.GetExportName()
	fhandler, err = os.Create(filepath.Join(exportPath, exportName))

	if err != nil {
		log.Fatal(err)
	}
	defer fhandler.Close()
	if dataset.Data_stream == nil {
		log.Fatal("no data stream found")
	}
	nofBytesWritten, err = fhandler.Write(dataset.Data_stream.Data.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	msg := fmt.Sprintf("Exported %s to %s", exportName, exportPath)
	logger.MTFlogger.Info(msg)
	fmt.Printf(msg + "\n")

	return nofBytesWritten
}

func (media_header Media_Header) showInfo() {
	fmt.Printf("Software Name: %s | Media Name: %s | Media Description: %s | Media Date %s\n",
		media_header.Info["SoftwareName"], media_header.Info["MediaName"], media_header.Info["MediaDescription"],
		media_header.Info["MediaData"])
}

func (data_set DataSet) showInfo() {
	fmt.Printf("Data Set Name: %s | Data Set Description %s | Username %s Write Date %s\n",
		data_set.Info["DataSetName"], data_set.Info["DataSetDescription"],
		data_set.Info["UserName"], data_set.Info["MediaWriteDate"])
}
