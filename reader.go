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
	FileMark *dblk.MTF_SFMB
}

type DataSet struct {
	MTF_SSET        *dblk.MTF_SSET
	Generic_streams []*dblk.GENERIC_STREAM
	Data_stream     *dblk.DATA_STREAM
	Pad_stream      *dblk.PAD_STREAM
	MTF_VOLB        *dblk.MTF_VOLB
}

func (dataset *DataSet) AppendData(data []byte) {
	dataset.Data_stream.AppendData(data)
}

func main() {
	filePath := flag.String("mtf", "", "path to microsoft tape archive")
	loggerActive := flag.Bool("log", false, "enable logging")

	now := time.Now()
	logfilename := "logs" + now.Format("2006-01-02T15_04_05") + ".txt"
	logger.InitializeLogger(*loggerActive, logfilename)

	flag.Parse()

	fhadler, err := os.Open(*filePath)
	if err != nil {

		log.Fatal(err)
	}

	offset := int64(0)

	buffer := make([]byte, 100*1024)

	var media_header Media_Header
	var data_set DataSet

	latest_attribute := ""
	for {
		_, err = fhadler.ReadAt(buffer, offset)
		if err != nil {
			log.Fatal(err)
			break
		}

		if latest_attribute == "MQDA" { //continue
			data_set.AppendData(buffer)
			offset += int64(len(buffer))
		}

		for offset < int64(len(buffer)) {

			if string(buffer[offset:offset+4]) == "TAPE" {

				mtf_tape := new(dblk.MTF_Tape)
				next_offset, err := mtf_tape.Parse(buffer[offset:])

				media_header.Tape = mtf_tape
				offset += next_offset
				if err != nil {

					break
				}

			} else if string(buffer[offset:offset+4]) == "SFMB" {

				mtf_sfmb := new(dblk.MTF_SFMB)
				next_offset, err := mtf_sfmb.Parse(buffer[offset:])

				media_header.FileMark = mtf_sfmb
				offset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[offset:offset+4]) == "SSET" {

				mtf_sset := new(dblk.MTF_SSET)
				next_offset, err := mtf_sset.Parse(buffer[offset:])

				data_set.MTF_SSET = mtf_sset
				offset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[offset:offset+4]) == "VOLB" {

				mtf_volb := new(dblk.MTF_VOLB)
				next_offset, err := mtf_volb.Parse(buffer[offset:])

				data_set.MTF_VOLB = mtf_volb
				offset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[offset:offset+4]) == "SPAD" {
				pad_stream := new(dblk.PAD_STREAM)
				next_offset, err := pad_stream.Parse(buffer[offset:])

				data_set.Pad_stream = pad_stream
				offset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[offset:offset+4]) == "RAID" {
				raid_stream := new(dblk.RAID_STREAM)
				next_offset, err := raid_stream.Parse(buffer[offset:])

				offset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[offset:offset+4]) == "MSCI" || string(buffer[offset:offset+4]) == "MSDA" {
				mtf_generic := new(dblk.MTF_Generic)
				next_offset, err := mtf_generic.Parse(buffer[offset:])

				offset += next_offset
				if err != nil {

					break
				}

			} else if string(buffer[offset:offset+4]) == "MQCI" || string(buffer[offset:offset+4]) == "APAD" ||
				string(buffer[offset:offset+4]) == "CSUM" {
				generic_stream := new(dblk.GENERIC_STREAM)
				next_offset, err := generic_stream.Parse(buffer[offset:])

				data_set.Generic_streams = append(data_set.Generic_streams, generic_stream)
				offset += next_offset
				if err != nil {
					break
				}

			} else if string(buffer[offset:offset+4]) == "MQDA" {
				data_stream := new(dblk.DATA_STREAM)
				next_offset, err := data_stream.Parse(buffer[offset:])

				data_set.Data_stream = data_stream
				offset += next_offset
				if err != nil {
					latest_attribute = "MQDA"
					break
				}
			} else {
				offset += 1 //brute force search alignment??
				logger.MTFlogger.Warning(fmt.Sprintf("Bruteforcing %d", offset))
			}

		}

	}

}
