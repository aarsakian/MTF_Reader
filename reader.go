package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aarsakian/MTF_Reader/dblk"
	"github.com/aarsakian/MTF_Reader/logger"
)

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

func (dataset *DataSet) AppendData(data []byte) int64 {
	return dataset.Data_stream.AppendData(data)
}

func (dataset DataSet) IsFull() bool {
	return dataset.Data_stream.IsFull()
}

func (dataset DataSet) Export() {
	var err error
	var fhandler *os.File
	nofBytesWritten := 0
	fhandler, err = os.Create(dataset.Info["DataSetName"])
	if err != nil {
		log.Fatal(err)
	}

	nofBytesWritten, err = fhandler.Write(dataset.Data_stream.Data.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Written %d\n", nofBytesWritten)
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

func main() {
	filePath := flag.String("mtf", "", "path to microsoft tape archive")
	loggerActive := flag.Bool("log", false, "enable logging")
	info := flag.Bool("info", false, "show info about the tape file")
	export := flag.Bool("export", false, "export data set of the tape file")

	flag.Parse()

	now := time.Now()
	logfilename := "logs" + now.Format("2006-01-02T15_04_05") + ".txt"
	logger.InitializeLogger(*loggerActive, logfilename)

	fhadler, err := os.Open(*filePath)
	if err != nil {

		log.Fatal(err)
	}

	offset := int64(0)

	buffer := make([]byte, 100000*1024)

	fsize, err := fhadler.Stat()
	if err != nil {
		logger.MTFlogger.Error(err)

	}

	var media_header Media_Header
	var data_set DataSet

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

	if *info {

		media_header.showInfo()
		data_set.showInfo()
	}

	if *export {
		data_set.Export()
	}

}
