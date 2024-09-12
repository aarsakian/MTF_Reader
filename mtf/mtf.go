package mtf

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aarsakian/MTF_Reader/dblk"
	"github.com/aarsakian/MTF_Reader/logger"
)

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
	fhadler, err := os.Open(mtf.Fname)
	if err != nil {

		log.Fatal(err)
	}

	offset := int64(0)

	buffer := make([]byte, 100000*1024)

	fsize, err := fhadler.Stat()
	if err != nil {
		logger.MTFlogger.Error(err)

	}

	media_header := new(Media_Header)
	data_set := new(DataSet)

	latest_attribute := ""
	for offset < fsize.Size() {
		_, err = fhadler.ReadAt(buffer, offset)
		if err != nil {
			logger.MTFlogger.Error(err)
			break
		}

		innerOffset := int64(0)
		for innerOffset < int64(len(buffer)) {
			if string(buffer[innerOffset:innerOffset+4]) == "TAPE" {

				mtf_tape := new(dblk.MTF_Tape)
				next_offset, err := mtf_tape.Parse(buffer[innerOffset:])

				media_header.Info = mtf_tape.GetInfo(buffer[innerOffset:])
				media_header.Tape = mtf_tape
				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[innerOffset:innerOffset+4]) == "SFMB" {

				mtf_sfmb := new(dblk.MTF_SFMB)
				next_offset, err := mtf_sfmb.Parse(buffer[innerOffset:])

				media_header.FileMark = mtf_sfmb
				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[innerOffset:innerOffset+4]) == "SSET" {

				mtf_sset := new(dblk.MTF_SSET)
				next_offset, err := mtf_sset.Parse(buffer[innerOffset:])

				data_set.Info = mtf_sset.GetInfo(buffer[innerOffset:])
				data_set.MTF_SSET = mtf_sset
				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[innerOffset:innerOffset+4]) == "VOLB" {

				mtf_volb := new(dblk.MTF_VOLB)
				next_offset, err := mtf_volb.Parse(buffer[innerOffset:])

				data_set.MTF_VOLB = mtf_volb
				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[innerOffset:innerOffset+4]) == "SPAD" {
				pad_stream := new(dblk.PAD_STREAM)
				next_offset, err := pad_stream.Parse(buffer[innerOffset:])

				data_set.Pad_stream = pad_stream
				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[innerOffset:innerOffset+4]) == "RAID" {
				raid_stream := new(dblk.RAID_STREAM)
				next_offset, err := raid_stream.Parse(buffer[innerOffset:])

				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[innerOffset:innerOffset+4]) == "MSCI" || string(buffer[innerOffset:innerOffset+4]) == "MSDA" {
				mtf_generic := new(dblk.MTF_Generic)
				next_offset, err := mtf_generic.Parse(buffer[innerOffset:])

				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[innerOffset:innerOffset+4]) == "MQCI" || string(buffer[innerOffset:innerOffset+4]) == "APAD" ||
				string(buffer[innerOffset:innerOffset+4]) == "CSUM" {
				generic_stream := new(dblk.GENERIC_STREAM)
				next_offset, err := generic_stream.Parse(buffer[innerOffset:])

				data_set.Generic_streams = append(data_set.Generic_streams, generic_stream)
				innerOffset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[innerOffset:innerOffset+4]) == "MQDA" {
				data_stream := new(dblk.DATA_STREAM)
				next_offset, err := data_stream.Parse(buffer[innerOffset:])

				data_set.Data_stream = data_stream
				innerOffset += next_offset
				if err != nil {
					latest_attribute = "MQDA"
					break
				}
			} else if latest_attribute == "MQDA" && !data_set.IsFull() { //break
				innerOffset += data_set.AppendData(buffer)

			} else {
				innerOffset += 1 //brute force search alignment??
				logger.MTFlogger.Warning(fmt.Sprintf("Bruteforcing %d", offset))
			}
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
	return strings.Replace(mtf.DataSet.Info["DataSetName"], " ", "_", -1) + ".mdf"
}

func (dataset DataSet) Export(exportPath string) int {
	var err error

	err = os.Mkdir(exportPath, 0750)
	if err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}
	var fhandler *os.File
	nofBytesWritten := 0
	exportName := strings.Replace(dataset.Info["DataSetName"], " ", "_", -1) + ".mdf"
	fhandler, err = os.Create(filepath.Join(exportPath, exportName))

	if err != nil {
		log.Fatal(err)
	}

	nofBytesWritten, err = fhandler.Write(dataset.Data_stream.Data.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	msg := fmt.Sprintf("Exported %s to %s", exportPath, exportName)
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
