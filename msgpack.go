package msgpack

import (
	"bytes"
	"encoding/binary"
	"io"
	"reflect"
	"strconv"
	"unsafe"
)

const (
	TYPE_FIXMAP  = 0x80
	TYPE_FIXARY  = 0x90
	TYPE_FIXRAW  = 0xa0
	TYPE_NIL     = 0xc0
	TYPE_FALSE   = 0xc2
	TYPE_TRUE    = 0xc3
	TYPE_FLOAT32 = 0xca
	TYPE_FLOAT64 = 0xcb
	TYPE_UINT8   = 0xcc
	TYPE_UINT16  = 0xcd
	TYPE_UINT32  = 0xce
	TYPE_UINT64  = 0xcf
	TYPE_INT8    = 0xd0
	TYPE_INT16   = 0xd1
	TYPE_INT32   = 0xd2
	TYPE_INT64   = 0xd3
	TYPE_RAW16   = 0xda
	TYPE_RAW32   = 0xdb
	TYPE_ARY16   = 0xdc
	TYPE_ARY32   = 0xdd
	TYPE_MAP16   = 0xde
	TYPE_MAP32   = 0xdf
	TYPE_FIXNEG  = 0xe0
)

type Encoder struct {
	io.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w}
}

func (enc *Encoder) Encode(value interface{}) (err error) {
	defer func() { err, _ = recover().(error) }()
	enc.encode(reflect.ValueOf(value))
	return
}

func (enc *Encoder) WriteByte(c byte) {
	if _, err := enc.Write([]byte{c}); err != nil {
		panic(err)
	}
}

func (enc *Encoder) WriteBinary(value interface{}) {
	if err := binary.Write(enc, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func (enc *Encoder) encode(value reflect.Value) {
	if !value.IsValid() {
		enc.encodeNil()
		return
	}

	switch value.Kind() {
	case reflect.Bool:
		enc.encodeBool(value.Bool())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		enc.encodeUint(value.Uint())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		enc.encodeInt(value.Int())
	case reflect.Float32, reflect.Float64:
		enc.encodeFloat(value.Float())
	case reflect.String:
		enc.encodeRaw([]byte(value.String()))
	case reflect.Array, reflect.Slice:
		enc.encodeArray(value)
	case reflect.Map:
		enc.encodeMap(value)
	case reflect.Struct:
		enc.encodeStruct(value)
	case reflect.Interface, reflect.Ptr:
		if value.IsNil() {
			enc.encodeNil()
		} else {
			enc.encode(value.Elem())
		}
	}
}

func (enc *Encoder) encodeNil() {
	enc.WriteByte(TYPE_NIL)
}

func (enc *Encoder) encodeBool(value bool) {
	if value {
		enc.WriteByte(TYPE_TRUE)
	} else {
		enc.WriteByte(TYPE_FALSE)
	}
}

func (enc *Encoder) encodeFloat(value float64) {
	enc.WriteByte(TYPE_FLOAT64)
	enc.WriteBinary(*(*uint64)(unsafe.Pointer(&value)))
}

func (enc *Encoder) encodeUint(value uint64) {
	switch {
	case value <= 127:
		enc.WriteByte(byte(value))
	case value <= 255:
		enc.WriteByte(TYPE_UINT8)
		enc.WriteByte(byte(value))
	case value <= 65535:
		enc.WriteByte(TYPE_UINT16)
		enc.WriteBinary(uint16(value))
	case value <= 4294967295:
		enc.WriteByte(TYPE_UINT32)
		enc.WriteBinary(uint32(value))
	default:
		enc.WriteByte(TYPE_UINT64)
		enc.WriteBinary(value)
	}
}

func (enc *Encoder) encodeInt(value int64) {
	switch {
	case -32 <= value && value <= -1:
		enc.WriteByte(TYPE_FIXNEG | byte(32+value))
	case -128 <= value && value <= 127:
		enc.WriteByte(TYPE_INT8)
		enc.WriteByte(byte(value))
	case -32768 <= value && value <= 32767:
		enc.WriteByte(TYPE_INT16)
		enc.WriteBinary(int16(value))
	case -2147483648 <= value && value <= 2147483647:
		enc.WriteByte(TYPE_INT32)
		enc.WriteBinary(int32(value))
	default:
		enc.WriteByte(TYPE_INT64)
		enc.WriteBinary(value)
	}
}

func (enc *Encoder) encodeRaw(value []byte) {
	length := len(value)

	switch {
	case length <= 31:
		enc.WriteByte(TYPE_FIXRAW | uint8(length))
	case length <= 65535:
		enc.WriteByte(TYPE_RAW16)
		enc.WriteBinary(uint16(length))
	default:
		enc.WriteByte(TYPE_RAW32)
		enc.WriteBinary(uint32(length))
	}

	enc.Write(value)
}

func (enc *Encoder) encodeArray(value reflect.Value) {
	if value.Type().Elem().Kind() == reflect.Uint8 {
		enc.encodeRaw(value.Interface().([]byte))
		return
	}

	length := value.Len()

	switch {
	case length <= 15:
		enc.WriteByte(TYPE_FIXARY | uint8(length))
	case length <= 65535:
		enc.WriteByte(TYPE_ARY16)
		enc.WriteBinary(uint16(length))
	default:
		enc.WriteByte(TYPE_ARY32)
		enc.WriteBinary(uint32(length))
	}

	for i := 0; i < length; i++ {
		enc.encode(value.Index(i))
	}
}

func (enc *Encoder) encodeMap(value reflect.Value) {
	length := value.Len()

	switch {
	case length <= 15:
		enc.WriteByte(TYPE_FIXMAP | uint8(length))
	case length <= 65535:
		enc.WriteByte(TYPE_MAP16)
		enc.WriteBinary(uint16(length))
	default:
		enc.WriteByte(TYPE_MAP32)
		enc.WriteBinary(uint32(length))
	}

	for _, key := range value.MapKeys() {
		enc.encode(key)
		enc.encode(value.MapIndex(key))
	}
}

func (enc *Encoder) encodeStruct(value reflect.Value) {
	fields := make(map[string]interface{})
	typeinfo := value.Type()
	length := typeinfo.NumField()

	for i := 0; i < length; i++ {
		field := typeinfo.Field(i)
		key := field.Tag.Get("msgpack")

		if field.PkgPath != "" || field.Anonymous {
			continue
		}

		switch key {
		case "-":
			continue
		case "":
			key = field.Name
		}

		fields[key] = value.Field(i).Interface()
	}

	enc.Encode(fields)
}

type Decoder struct {
	io.Reader
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r}
}

func (dec *Decoder) Decode(ptr interface{}) (err error) {
	defer func() { err, _ = recover().(error) }()
	dec.decode(reflect.ValueOf(ptr).Elem())
	return
}

func (dec *Decoder) ReadByte() byte {
	var buf [1]byte

	if _, err := dec.Read(buf[:]); err != nil {
		panic(err)
	}

	return buf[0]
}

func (dec *Decoder) ReadBinary(ptr interface{}) {
	if err := binary.Read(dec, binary.BigEndian, ptr); err != nil {
		panic(err)
	}
}

func (dec *Decoder) decode(ptr_value reflect.Value) {
	typeid := dec.ReadByte()

	switch {
	case typeid <= 0x7f:
		typeid = TYPE_UINT8
		dec.decodeUint(ptr_value, uint64(typeid))
	case typeid <= TYPE_FIXMAP+15:
		dec.decodeMap(ptr_value, uint32(typeid-TYPE_FIXMAP))
	case typeid <= TYPE_FIXARY+15:
		dec.decodeArray(ptr_value, uint32(typeid-TYPE_FIXARY))
	case typeid <= TYPE_FIXRAW+31:
		dec.decodeRaw(ptr_value, uint32(typeid-TYPE_FIXRAW))
	case typeid >= TYPE_FIXNEG:
		dec.decodeInt(ptr_value, int64(int8(typeid)))
	}

	switch typeid {
	case TYPE_NIL:
		dec.decodeNil(ptr_value)
	case TYPE_FALSE:
		dec.decodeBool(ptr_value, false)
	case TYPE_TRUE:
		dec.decodeBool(ptr_value, true)
	case TYPE_UINT8:
		dec.decodeUint(ptr_value, uint64(dec.ReadByte()))
	case TYPE_INT8:
		dec.decodeInt(ptr_value, int64(int8(dec.ReadByte())))
	case TYPE_UINT16, TYPE_INT16:
		var value uint16
		dec.ReadBinary(&value)

		switch typeid {
		case TYPE_UINT16:
			dec.decodeUint(ptr_value, uint64(value))
		case TYPE_INT16:
			dec.decodeInt(ptr_value, int64(int16(value)))
		}
	case TYPE_FLOAT32, TYPE_UINT32, TYPE_INT32:
		var value uint32
		dec.ReadBinary(&value)

		switch typeid {
		case TYPE_FLOAT32:
			dec.decodeFloat(ptr_value, float64(*(*uint32)(unsafe.Pointer(&value))))
		case TYPE_UINT32:
			dec.decodeUint(ptr_value, uint64(value))
		case TYPE_INT32:
			dec.decodeInt(ptr_value, int64(int32(value)))
		}
	case TYPE_FLOAT64, TYPE_UINT64, TYPE_INT64:
		var value uint64
		dec.ReadBinary(&value)

		switch typeid {
		case TYPE_FLOAT64:
			dec.decodeFloat(ptr_value, *(*float64)(unsafe.Pointer(&value)))
		case TYPE_UINT64:
			dec.decodeUint(ptr_value, value)
		case TYPE_INT64:
			dec.decodeInt(ptr_value, int64(value))
		}
	case TYPE_MAP16, TYPE_ARY16, TYPE_RAW16:
		var length uint16
		dec.ReadBinary(&length)

		switch typeid {
		case TYPE_MAP16:
			dec.decodeMap(ptr_value, uint32(length))
		case TYPE_ARY16:
			dec.decodeArray(ptr_value, uint32(length))
		case TYPE_RAW16:
			dec.decodeRaw(ptr_value, uint32(length))
		}
	case TYPE_MAP32, TYPE_ARY32, TYPE_RAW32:
		var length uint32
		dec.ReadBinary(&length)

		switch typeid {
		case TYPE_MAP32:
			dec.decodeMap(ptr_value, length)
		case TYPE_ARY32:
			dec.decodeArray(ptr_value, length)
		case TYPE_RAW32:
			dec.decodeRaw(ptr_value, length)
		}
	}
}

func (dec *Decoder) decodeNil(ptr_value reflect.Value) {
	ptr_value.Set(reflect.Zero(ptr_value.Type()))
}

func (dec *Decoder) decodeBool(ptr_value reflect.Value, value bool) {
	switch ptr_value.Kind() {
	case reflect.Bool:
		ptr_value.SetBool(value)
	case reflect.String:
		ptr_value.SetString(strconv.FormatBool(value))
	case reflect.Interface:
		ptr_value.Set(reflect.ValueOf(value))
	}
}

func (dec *Decoder) decodeFloat(ptr_value reflect.Value, value float64) {
	switch ptr_value.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		ptr_value.SetUint(uint64(value))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		ptr_value.SetInt(int64(value))
	case reflect.Float32, reflect.Float64:
		ptr_value.SetFloat(value)
	case reflect.String:
		ptr_value.SetString(strconv.FormatFloat(value, 'g', -1, 64))
	case reflect.Interface:
		ptr_value.Set(reflect.ValueOf(value))
	}
}

func (dec *Decoder) decodeUint(ptr_value reflect.Value, value uint64) {
	switch ptr_value.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		ptr_value.SetUint(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		ptr_value.SetInt(int64(value))
	case reflect.Float32, reflect.Float64:
		ptr_value.SetFloat(float64(value))
	case reflect.String:
		ptr_value.SetString(strconv.FormatUint(value, 10))
	case reflect.Interface:
		ptr_value.Set(reflect.ValueOf(value))
	}
}

func (dec *Decoder) decodeInt(ptr_value reflect.Value, value int64) {
	switch ptr_value.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		ptr_value.SetUint(uint64(value))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		ptr_value.SetInt(value)
	case reflect.Float32, reflect.Float64:
		ptr_value.SetFloat(float64(value))
	case reflect.String:
		ptr_value.SetString(strconv.FormatInt(value, 10))
	case reflect.Interface:
		ptr_value.Set(reflect.ValueOf(value))
	}
}

func (dec *Decoder) decodeRaw(ptr_value reflect.Value, length uint32) {
	value := make([]byte, length)

	dec.Read(value)

	switch ptr_value.Kind() {
	case reflect.String:
		ptr_value.SetString(string(value))
	case reflect.Slice:
		ptr_value.SetBytes(value)
	case reflect.Interface:
		ptr_value.Set(reflect.ValueOf(string(value)))
	}
}

func (dec *Decoder) decodeArray(ptr_value reflect.Value, length uint32) {
	var value reflect.Value

	switch ptr_value.Kind() {
	case reflect.Slice:
		value = reflect.MakeSlice(ptr_value.Type(), int(length), int(length))
	case reflect.Interface:
		value = reflect.ValueOf(make([]interface{}, int(length)))
	}

	for i := 0; i < int(length); i++ {
		dec.Decode(value.Index(i).Addr().Interface())
	}

	ptr_value.Set(value)
}

func (dec *Decoder) decodeMap(ptr_value reflect.Value, length uint32) {
	var value reflect.Value

	switch ptr_value.Kind() {
	case reflect.Map:
		value = reflect.MakeMap(ptr_value.Type())
	case reflect.Struct:
		dec.decodeStruct(ptr_value, length)
		return
	case reflect.Interface:
		value = reflect.ValueOf(make(map[interface{}]interface{}))
	}

	for i := 0; i < int(length); i++ {
		key_ptr := reflect.New(value.Type().Key())
		value_ptr := reflect.New(value.Type().Elem())
		dec.Decode(key_ptr.Interface())
		dec.Decode(value_ptr.Interface())
		value.SetMapIndex(key_ptr.Elem(), value_ptr.Elem())
	}

	ptr_value.Set(value)
}

func (dec *Decoder) decodeStruct(ptr_value reflect.Value, map_len uint32) {
	typeinfo := ptr_value.Type()
	fields_len := typeinfo.NumField()
	fields := make(map[string]reflect.Value)

	for i := 0; i < fields_len; i++ {
		field := typeinfo.Field(i)
		key := field.Tag.Get("msgpack")

		if field.PkgPath != "" || field.Anonymous {
			continue
		}

		switch key {
		case "-":
			continue
		case "":
			key = field.Name
		}

		fields[key] = ptr_value.Field(i)
	}

	for i := 0; i < int(map_len); i++ {
		var key string
		dec.Decode(&key)

		value_ptr := reflect.New(fields[key].Type())
		dec.Decode(value_ptr.Interface())

		if field, ok := fields[key]; ok {
			field.Set(value_ptr.Elem())
		}
	}
}

func Marshal(value interface{}) ([]byte, error) {
	buffer := new(bytes.Buffer)

	err := NewEncoder(buffer).Encode(value)

	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func Unmarshal(data []byte, ptr interface{}) error {
	buffer := bytes.NewBuffer(data)
	return NewDecoder(buffer).Decode(ptr)
}