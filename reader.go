package main

import (
	"flag"
	"time"

	"github.com/aarsakian/MTF_Reader/logger"
	"github.com/aarsakian/MTF_Reader/mtf"
)

func main() {
	filePath := flag.String("mtf", "", "path to microsoft tape archive")
	loggerActive := flag.Bool("log", false, "enable logging")
	info := flag.Bool("info", false, "show info about the tape file")
	exportPath := flag.String("export", "", "export path of the data set of the tape file")

	flag.Parse()

	now := time.Now()
	logfilename := "logs" + now.Format("2006-01-02T15_04_05") + ".txt"
	logger.InitializeLogger(*loggerActive, logfilename)

	mtf_s := mtf.MTF{Fname: *filePath}
	mtf_s.Process()

	if *info {
		mtf_s.ShowInfo()
	}

	if *exportPath != "" {
		mtf_s.Export(*exportPath)
	}

}
