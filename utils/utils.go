package utils

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
)

type MTF_DATE_TIME struct {
	Content [5]byte
}

type MTF_TAPE_ADDRESS struct {
	Size   uint16
	Offset uint16
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
