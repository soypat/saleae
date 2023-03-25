package saleae

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"unsafe"
)

const (
	fileHeaderSize    = unsafe.Sizeof(FileHeader{})
	digitalHeaderSize = 44 //unsafe.Sizeof(DigitalHeader{})
	analogHeaderSize  = unsafe.Sizeof(AnalogHeader{})
)

type FileType int32

const (
	FileTypeDigital = 0
	FileTypeAnalog  = 1
)

var expectID = [8]byte{'<', 'S', 'A', 'L', 'E', 'A', 'E', '>'}

type FileHeader struct {
	ID      [8]byte
	Version int32
	Type    FileType
}

func (fh *FileHeader) Validate() error {
	if fh.ID != expectID {
		return fmt.Errorf("expected file header id <SALEAE>, got %q", fh.ID)
	}
	if fh.Version != 0 {
		return fmt.Errorf("expected file header version 0, got %d", fh.Version)
	}
	if fh.Type != 0 {
		return fmt.Errorf("expected file header type in range 0..1, got %d", fh.Type)
	}
	return nil
}

func (fh *FileHeader) array() *[16]byte { // Use const literal so breaks at compile time if size mismatches
	return (*[fileHeaderSize]byte)(unsafe.Pointer(fh))
}

func decodeFileHeader(b []byte) (fh FileHeader, n int) {
	_ = b[fileHeaderSize-1]
	n = copy(fh.array()[:], b)
	return fh, n
}

func (fh *FileHeader) put(b []byte) int {
	_ = b[fileHeaderSize-1]
	return copy(b, fh.array()[:])
}

type DigitalHeader struct {
	Info           FileHeader
	InitialState   uint32
	Begin          float64
	End            float64
	NumTransitions uint64
}

func (dh *DigitalHeader) put(b []byte) int {
	_ = b[digitalHeaderSize-1]
	n := dh.Info.put(b)
	binary.LittleEndian.PutUint32(b[n:], dh.InitialState)
	n += 4
	binary.LittleEndian.PutUint64(b[n:], math.Float64bits(dh.Begin))
	n += 8
	binary.LittleEndian.PutUint64(b[n:], math.Float64bits(dh.End))
	n += 8
	binary.LittleEndian.PutUint64(b[n:], dh.NumTransitions)
	n += 8
	return n
}

func decodeDigitalHeader(b []byte) (dh DigitalHeader, n int) {
	_ = b[digitalHeaderSize-1]
	dh.Info, n = decodeFileHeader(b)
	dh.InitialState = binary.LittleEndian.Uint32(b[n:])
	n += 4
	dh.Begin = math.Float64frombits(binary.LittleEndian.Uint64(b[n:]))
	n += 8
	dh.End = math.Float64frombits(binary.LittleEndian.Uint64(b[n:]))
	n += 8
	dh.NumTransitions = binary.LittleEndian.Uint64(b[n:])
	n += 8
	return dh, n
}

type AnalogFile struct {
	Header AnalogHeader
	// Voltage readings.
	Data []float64
}

type AnalogHeader struct {
	Info       FileHeader
	Begin      float64
	SampleRate uint64
	Downsample uint64
	NumSamples uint64
}

func (ah *AnalogHeader) put(b []byte) int {
	_ = b[digitalHeaderSize-1]
	n := ah.Info.put(b)
	binary.LittleEndian.PutUint64(b[n:], math.Float64bits(ah.Begin))
	n += 8
	binary.LittleEndian.PutUint64(b[n:], ah.SampleRate)
	n += 8
	binary.LittleEndian.PutUint64(b[n:], ah.Downsample)
	n += 8
	binary.LittleEndian.PutUint64(b[n:], ah.NumSamples)
	n += 8
	return n
}

func decodeAnalogHeader(b []byte) (ah AnalogHeader, n int) {
	_ = b[digitalHeaderSize-1]
	ah.Info, n = decodeFileHeader(b)
	// n := fh.Info.binGet(b)
	ah.Begin = math.Float64frombits(binary.LittleEndian.Uint64(b[n:]))
	n += 8
	ah.SampleRate = binary.LittleEndian.Uint64(b[n:])
	n += 8
	ah.Downsample = binary.LittleEndian.Uint64(b[n:])
	n += 8
	ah.NumSamples = binary.LittleEndian.Uint64(b[n:])
	n += 8
	return ah, n
}

type DigitalFile struct {
	Header DigitalHeader
	// Times at which transitions happened
	Data []float64
}

func ReadDigitalFile(r io.Reader) (*DigitalFile, error) {
	if r == nil {
		return nil, errors.New("got nil reader")
	}
	var buf [digitalHeaderSize]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return nil, err
	}
	var file DigitalFile
	dh, n := decodeDigitalHeader(buf[:])
	if n != len(buf) {
		panic("bad buffer length")
	}
	file.Header = dh
	err = file.Header.Info.Validate()
	if err != nil {
		return nil, err
	}
	if file.Header.Info.Version != FileTypeDigital {
		return nil, errors.New("file type mismatch, expected 0, got " + strconv.Itoa(int(file.Header.Info.Version)))
	}
	file.Data = make([]float64, file.Header.NumTransitions)
	databuf := unsafe.Slice((*byte)(unsafe.Pointer(&file.Data[0])), len(file.Data)*8)
	_, err = io.ReadFull(r, databuf)
	if err != nil {
		return nil, err
	}
	return &file, nil
}

func ReadAnalogFile(r io.Reader) (*AnalogFile, error) {
	if r == nil {
		return nil, errors.New("got nil reader")
	}
	var buf [analogHeaderSize]byte
	_, err := io.ReadFull(r, buf[:])
	if err != nil {
		return nil, err
	}
	var file AnalogFile
	ah, n := decodeAnalogHeader(buf[:])
	if n != len(buf) {
		panic("bad buffer length")
	}
	file.Header = ah
	err = file.Header.Info.Validate()
	if err != nil {
		return nil, err
	}
	if file.Header.Info.Version != FileTypeAnalog {
		return nil, errors.New("file type mismatch, expected 1, got " + strconv.Itoa(int(file.Header.Info.Version)))
	}
	file.Data = make([]float64, file.Header.NumSamples)
	databuf := unsafe.Slice((*byte)(unsafe.Pointer(&file.Data[0])), len(file.Data)*8)
	_, err = io.ReadFull(r, databuf)
	if err != nil {
		return nil, err
	}
	return &file, nil
}

func (af *AnalogFile) WriteTo(w io.Writer) (int64, error) {
	var buf [analogHeaderSize]byte
	n := af.Header.put(buf[:])
	if n != len(buf) {
		panic("bad length")
	}
	n, err := w.Write(buf[:])
	if err != nil {
		return int64(n), err
	}
	databuf := unsafe.Slice((*byte)(unsafe.Pointer(&af.Data[0])), len(af.Data)*8)
	n2, err := w.Write(databuf)
	if err != nil {
		return int64(n2 + n), err
	}
	if n2 != len(af.Data)*8 {
		return int64(n2 + n), errors.New("bad writer implementation")
	}
	return int64(n2 + n), nil
}

func (df *DigitalFile) WriteTo(w io.Writer) (int64, error) {
	var buf [digitalHeaderSize]byte
	n := df.Header.put(buf[:])
	if n != len(buf) {
		panic("bad length")
	}
	n, err := w.Write(buf[:])
	if err != nil {
		return int64(n), err
	}
	databuf := unsafe.Slice((*byte)(unsafe.Pointer(&df.Data[0])), len(df.Data)*8)
	n2, err := w.Write(databuf)
	if err != nil {
		return int64(n2 + n), err
	}
	if n2 != len(databuf) {
		return int64(n2 + n), errors.New("bad writer implementation")
	}
	return int64(n2 + n), nil
}
