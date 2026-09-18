package mtf

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const pageLen = 8192

func setChecksum(hdr []byte, checksumOffset int) {
	var checksum uint16
	for i := 0; i < checksumOffset; i += 2 {
		checksum ^= binary.LittleEndian.Uint16(hdr[i:])
	}
	binary.LittleEndian.PutUint16(hdr[checksumOffset:], checksum)
}

func putDBLK(data []byte, dblkType string, firstEventOffset uint16) {
	copy(data, dblkType)
	binary.LittleEndian.PutUint16(data[8:], firstEventOffset)
	setChecksum(data, 50)
}

func putStream(data []byte, streamID string, length uint64) {
	copy(data, streamID)
	binary.LittleEndian.PutUint64(data[8:], length)
	setChecksum(data, 20)
}

// buildBackup lays out a backup the way SQL Server writes one: the database pages
// in the first MQDA stream, then an empty MQDA, a re-copy of a page, another
// empty MQDA, and finally a block name occurring inside a string.
func buildBackup() (backup []byte, pages []byte) {
	backup = make([]byte, 0x9000)

	putDBLK(backup[0x0:], "TAPE", 0x400)

	sset := backup[0x400:]
	name := []byte{'u', 0, 'n', 0, 'i', 0, 't', 0, ' ', 0, 't', 0, 'e', 0, 's', 0, 't', 0}
	copy(sset[0x100:], name)
	binary.LittleEndian.PutUint16(sset[64:], uint16(len(name))) // DataSetName size
	binary.LittleEndian.PutUint16(sset[66:], 0x100)             // DataSetName offset
	putDBLK(sset, "SSET", 0x400)

	pages = make([]byte, 2*pageLen)
	for i := range pages {
		pages[i] = byte(i % 251)
	}
	offset := 0x800
	putStream(backup[offset:], "MQDA", uint64(2+len(pages)))
	copy(backup[offset+24:], pages)
	offset += 24 + len(pages)

	putStream(backup[offset:], "MQDA", 0)
	offset += 22

	putStream(backup[offset:], "MQDA", 2+pageLen)
	copy(backup[offset+24:], bytes.Repeat([]byte{0xff}, pageLen))
	offset += 24 + pageLen

	putStream(backup[offset:], "MQDA", 0)

	copy(backup[0x7000:], "Backup device VOLB")
	return backup, pages
}

func writeBackup(t *testing.T, backup []byte) string {
	t.Helper()
	fname := filepath.Join(t.TempDir(), "test.bak")
	if err := os.WriteFile(fname, backup, 0600); err != nil {
		t.Fatal(err)
	}
	return fname
}

func process(t *testing.T, fname string) MTF {
	t.Helper()
	mtf := MTF{Fname: fname}
	done := make(chan struct{})
	go func() {
		mtf.Process()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Process did not finish")
	}
	return mtf
}

func TestProcessExportsDatabasePages(t *testing.T) {
	backup, pages := buildBackup()
	mtf := process(t, writeBackup(t, backup))

	if name := mtf.GetExportFileName(); name != "unit_test.mdf" {
		t.Errorf("export name %q, want %q", name, "unit_test.mdf")
	}
	exportPath := filepath.Join(t.TempDir(), "MDF")
	mtf.Export(exportPath)
	exported, err := os.ReadFile(filepath.Join(exportPath, mtf.GetExportFileName()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(exported, pages) {
		t.Errorf("exported %d bytes, want the %d bytes of the first MQDA stream", len(exported), len(pages))
	}
}

func TestProcessInChunks(t *testing.T) {
	backup, pages := buildBackup()
	fname := writeBackup(t, backup)

	defaultBufSize := BUF_SIZE
	defer func() { BUF_SIZE = defaultBufSize }()
	for _, bufSize := range []int64{1000, 4099, pageLen, 16411} {
		BUF_SIZE = bufSize
		mtf := process(t, fname)
		if !bytes.Equal(mtf.DataSet.Data_stream.Data.Bytes(), pages) {
			t.Errorf("buffer size %d: got %d bytes, want %d", bufSize, mtf.DataSet.Data_stream.Data.Len(), len(pages))
		}
	}
}

func TestProcessTruncatedBackup(t *testing.T) {
	backup, pages := buildBackup()
	for _, size := range []int{0x400 + 60, 0x800 + 24 + 100, 0x7000 + 20} {
		mtf := process(t, writeBackup(t, backup[:size]))
		if stream := mtf.DataSet.Data_stream; stream != nil && !bytes.HasPrefix(pages, stream.Data.Bytes()) {
			t.Errorf("size %d: data stream is not a prefix of the database pages", size)
		}
	}
}
