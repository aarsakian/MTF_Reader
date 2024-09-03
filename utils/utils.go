package utils

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type MTF_DATE_TIME struct {
	Content [5]byte
}

type MTF_TAPE_ADDRESS struct {
	Size   uint16
	Offset uint16
}

func (mtf_date_time MTF_DATE_TIME) ToString() string {
	bitrepresentation := strings.Builder{}
	bitrepresentation.Grow(8 * len(mtf_date_time.Content))
	//mtf_date_time.Content = [5]byte{0x1f, 0x33, 0x3f, 0x81, 0xde} //testing
	for _, val := range mtf_date_time.Content {

		bitrepresentation.WriteString(AddMissingBits(strconv.FormatUint(uint64(val), 2), 8))
	}
	year := 0
	bitrepresentationStr := bitrepresentation.String()
	for bitpos := range bitrepresentationStr[:14] {
		year += (int(bitrepresentationStr[14-bitpos-1]) - 48) * int(math.Exp2(float64(bitpos)))
	}

	month := 0
	for bitpos := range bitrepresentation.String()[14:18] {
		month += (int(bitrepresentationStr[18-bitpos-1]) - 48) * int(math.Exp2(float64(bitpos)))
	}

	day := 0
	for bitpos := range bitrepresentation.String()[18:23] {
		day += (int(bitrepresentationStr[23-bitpos-1]) - 48) * int(math.Exp2(float64(bitpos)))
	}

	hour := 0
	for bitpos := range bitrepresentation.String()[23:28] {
		hour += (int(bitrepresentationStr[28-bitpos-1]) - 48) * int(math.Exp2(float64(bitpos)))
	}

	minute := 0
	for bitpos := range bitrepresentation.String()[28:34] {
		minute += (int(bitrepresentationStr[34-bitpos-1]) - 48) * int(math.Exp2(float64(bitpos)))
	}
	seconds := 0
	for bitpos := range bitrepresentation.String()[34:40] {
		seconds += (int(bitrepresentationStr[40-bitpos-1]) - 48) * int(math.Exp2(float64(bitpos)))
	}

	return fmt.Sprintf("%d/%d/%d %d:%d:%d", day, month, year, hour, minute, seconds)

}

func ReadEndianInt(barray []byte) int64 {
	var buf []byte
	if barray[len(barray)-1]&0x80 != 0 { //check for sign
		buf = []byte{0xff, 0xff, 0xff, 0xff}
	} else {
		buf = []byte{0x00, 0x00, 0x00, 0x00}
	}

	var sum int32
	copy(buf, barray)

	binary.Read(bytes.NewBuffer(buf), binary.LittleEndian, &sum)
	return int64(sum)

}

func Unmarshal(data []byte, v interface{}) error {
	idx := 0
	structValPtr := reflect.ValueOf(v)
	structType := reflect.TypeOf(v)
	if structType.Elem().Kind() != reflect.Struct {
		return errors.New("must be a struct")
	}
	for i := 0; i < structValPtr.Elem().NumField(); i++ {
		field := structValPtr.Elem().Field(i) //StructField type
		switch field.Kind() {
		case reflect.String:
			name := structType.Elem().Field(i).Name
			if name == "MagicNumber" {
				field.SetString(string(Bytereverse(data[idx : idx+4])))
				idx += 4

			}

		case reflect.Struct:
			nameType := structType.Elem().Field(i).Type.Name()
			if nameType == "MTF_TAPE_ADDRESS" {

				var mtf_tape_address MTF_TAPE_ADDRESS
				Unmarshal(data[idx:idx+8], &mtf_tape_address)
				field.Set(reflect.ValueOf(mtf_tape_address))

				idx += 4
			} else if nameType == "MTF_DATE_TIME" {
				var mtf_date_time MTF_DATE_TIME
				Unmarshal(data[idx:idx+5], &mtf_date_time)
				field.Set(reflect.ValueOf(mtf_date_time))

				idx += 5
			}

		case reflect.Uint8:
			var temp uint8
			binary.Read(bytes.NewBuffer(data[idx:idx+1]), binary.LittleEndian, &temp)
			field.SetUint(uint64(temp))
			idx += 1
		case reflect.Uint16:
			var temp uint16
			binary.Read(bytes.NewBuffer(data[idx:idx+2]), binary.LittleEndian, &temp)
			field.SetUint(uint64(temp))
			idx += 2
		case reflect.Uint32:
			var temp uint32
			binary.Read(bytes.NewBuffer(data[idx:idx+4]), binary.LittleEndian, &temp)
			field.SetUint(uint64(temp))
			idx += 4
		case reflect.Uint64:
			var temp uint64

			binary.Read(bytes.NewBuffer(data[idx:idx+8]), binary.LittleEndian, &temp)
			idx += 8

			field.SetUint(temp)
		case reflect.Bool:
			field.SetBool(false)
			idx += 1
		case reflect.Array:
			arrT := reflect.ArrayOf(field.Len(), reflect.TypeOf(data[0])) //create array type to hold the slice
			arr := reflect.New(arrT).Elem()                               //initialize and access array
			var end int
			if idx+field.Len() > len(data) { //determine end
				end = len(data)
			} else {
				end = idx + field.Len()
			}
			for idx, val := range data[idx:end] {

				arr.Index(idx).Set(reflect.ValueOf(val))
			}

			field.Set(arr)
			idx += field.Len()

		}

	}
	return nil
}

func Bytereverse(barray []byte) []byte { //work with indexes
	//  fmt.Println("before",barray)
	for i, j := 0, len(barray)-1; i < j; i, j = i+1, j-1 {

		barray[i], barray[j] = barray[j], barray[i]

	}
	return barray

}

func DecodeUTF16(b []byte) string {
	utf := make([]uint16, (len(b)+(2-1))/2) // utf-16 2 bytes for each char
	for i := 0; i+(2-1) < len(b); i += 2 {
		utf[i/2] = binary.LittleEndian.Uint16(b[i:])
	}
	if len(b)/2 < len(utf) { // the "error" Rune or "Unicode replacement character"
		utf[len(utf)-1] = utf8.RuneError
	}
	return string(utf16.Decode(utf))

}

func AddMissingBits(bitval string, targetLen int) string {
	// add missing zeros

	for len(bitval) < targetLen {
		bitval = "0" + bitval
	}
	return bitval
}
