package saleae

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

const (
	fileHeaderSize    = 16 //  unsafe.Sizeof(FileHeader{})
	digitalHeaderSize = 44 //unsafe.Sizeof(DigitalHeader{})
	analogHeaderSize  = 48 //unsafe.Sizeof(AnalogHeader{})
)

type FileType int32

const (
	FileTypeDigital = 0
	FileTypeAnalog  = 1
)

var expectID = [8]byte{'<', 'S', 'A', 'L', 'E', 'A', 'E', '>'}

type FileHeader struct {
	Version int32
	Type    FileType
}

func (fh *FileHeader) Validate() error {
	if fh.Version != 0 {
		return fmt.Errorf("expected file header version 0, got %d", fh.Version)
	}
	if fh.Type != 0 {
		return fmt.Errorf("expected file header type in range 0..1, got %d", fh.Type)
	}
	return nil
}

func decodeFileHeader(b []byte) (fh FileHeader, n int, err error) {
	_ = b[fileHeaderSize-1]
	if !bytes.Equal(b[:8], expectID[:]) {
		return fh, 0, errors.New("invalid file header")
	}
	fh.Version = int32(binary.LittleEndian.Uint32(b[n+8:]))
	fh.Type = FileType(binary.LittleEndian.Uint32(b[n+12:]))
	return fh, fileHeaderSize, nil
}

func (fh *FileHeader) put(b []byte) int {
	_ = b[fileHeaderSize-1]
	n := copy(b, expectID[:])
	binary.LittleEndian.PutUint32(b[n:], uint32(fh.Version))
	n += 4
	binary.LittleEndian.PutUint32(b[n:], uint32(fh.Type))
	n += 4
	return n
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

func decodeDigitalHeader(b []byte) (dh DigitalHeader, n int, err error) {
	_ = b[digitalHeaderSize-1]
	dh.Info, n, err = decodeFileHeader(b)
	if err != nil {
		return dh, 0, err
	}
	dh.InitialState = binary.LittleEndian.Uint32(b[n:])
	n += 4
	dh.Begin = math.Float64frombits(binary.LittleEndian.Uint64(b[n:]))
	n += 8
	dh.End = math.Float64frombits(binary.LittleEndian.Uint64(b[n:]))
	n += 8
	dh.NumTransitions = binary.LittleEndian.Uint64(b[n:])
	n += 8
	return dh, n, nil
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

func decodeAnalogHeader(b []byte) (ah AnalogHeader, n int, err error) {
	_ = b[digitalHeaderSize-1]
	ah.Info, n, err = decodeFileHeader(b)
	if err != nil {
		return ah, 0, err
	}
	// n := fh.Info.binGet(b)
	ah.Begin = math.Float64frombits(binary.LittleEndian.Uint64(b[n:]))
	n += 8
	ah.SampleRate = binary.LittleEndian.Uint64(b[n:])
	n += 8
	ah.Downsample = binary.LittleEndian.Uint64(b[n:])
	n += 8
	ah.NumSamples = binary.LittleEndian.Uint64(b[n:])
	n += 8
	return ah, n, nil
}

// DigitalFile is a version 0 Saleae digital binary capture file.
type DigitalFile struct {
	Header DigitalHeader
	// Times at which transitions happened
	Data []float64
}

// ReadDigitalFile reads a Logic 2 version 0 Saleae digital file.
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
	dh, n, err := decodeDigitalHeader(buf[:])
	if err != nil {
		return nil, err
	}
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

// ReadAnalogFile reads a Logic 2 version 0 Saleae analog binary capture file.
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
	ah, n, err := decodeAnalogHeader(buf[:])
	if err != nil {
		return nil, err
	}
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

// WriteTo writes the file to w.
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

// WriteTo writes the digital file to w.
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

type Capture struct {
	CaptureStart time.Time
	AnalogFiles  []AnalogFile
	DigitalFiles []DigitalFile
}

// ReadCaptureFile reads a capture from a file in .sal format.
func ReadCaptureFile(path string) (*Capture, error) {
	fp, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	finfo, err := fp.Stat()
	if err != nil {
		return nil, err
	}
	size := finfo.Size()
	return ReadCapture(fp, size)
}

// ReadCapture reads a capture from a reader in .sal format. The reader must be seekable.
func ReadCapture(r io.ReaderAt, size int64) (*Capture, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	var metadata metadataV15
	for _, f := range zr.File {
		if f.Name != "meta.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		err = json.NewDecoder(rc).Decode(&metadata)
		rc.Close()
		if err != nil {
			return nil, err
		}
		break
	}
	if metadata.Version == 0 {
		return nil, errors.New("metadata.json not found or invalid version")
	}

	var capture Capture
	capture.CaptureStart = time.Unix(0, metadata.Data.CaptureStartTime.UnixTimeMilliseconds*int64(time.Millisecond)+int64(metadata.Data.CaptureStartTime.FractionalMilliseconds*float64(time.Millisecond)))
	for _, bindata := range metadata.BinData {
		if bindata.Type != "Analog" && bindata.Type != "Digital" {
			return nil, fmt.Errorf("unknown binary data type %q", bindata.Type)
		}

		filename := strings.TrimLeft(bindata.File, "./")
		fp, err := zr.Open(filename)
		if err != nil || fp == nil {
			return nil, fmt.Errorf("opening %s binary data %q: %w", bindata.Type, filename, err)
		}
		switch bindata.Type {
		case "Analog":
			af, err := ReadAnalogFile(fp)
			fp.Close()
			if err != nil {
				return nil, fmt.Errorf("reading analog file %q: %w", filename, err)
			}
			capture.AnalogFiles = append(capture.AnalogFiles, *af)
		case "Digital":
			df, err := ReadDigitalFile(fp)
			fp.Close()
			if err != nil {
				return nil, fmt.Errorf("reading digital file %q: %w", filename, err)
			}
			capture.DigitalFiles = append(capture.DigitalFiles, *df)
		}
	}
	return &capture, nil
}

type metadataV15 struct {
	Version int `json:"version"` // Always 15 for this case.
	Data    struct {
		RenderViewState struct {
			LeftEdgeTimeSec float64 `json:"leftEdgeTimeSec"`
			TimeSecPerPixel float64 `json:"timeSecPerPixel"`
		} `json:"renderViewState"`
		CaptureProgress struct {
			MaxCollectedTime  float64 `json:"maxCollectedTime"`
			ProcessedInterval struct {
				Begin int     `json:"begin"`
				End   float64 `json:"end"`
			} `json:"processedInterval"`
			MemoryUsedMb int  `json:"memoryUsedMb"`
			IsProcessing bool `json:"isProcessing"`
		} `json:"captureProgress"`
		CaptureStartTime struct {
			UnixTimeMilliseconds   int64   `json:"unixTimeMilliseconds"`
			FractionalMilliseconds float64 `json:"fractionalMilliseconds"`
		} `json:"captureStartTime"`
		TimingMarkers struct {
			Markers struct {
			} `json:"markers"`
			Pairs struct {
			} `json:"pairs"`
		} `json:"timingMarkers"`
		Measurements       []any `json:"measurements"`
		HighLevelAnalyzers []any `json:"highLevelAnalyzers"`
		Analyzers          []struct {
			Color        string `json:"color"`
			DisplayRadix string `json:"displayRadix"`
			Name         string `json:"name"`
			Settings     []struct {
				Title    string `json:"title"`
				Tooltip  string `json:"tooltip"`
				Disabled bool   `json:"disabled"`
				Setting  struct {
					Type            string `json:"type"`
					ChannelRequired bool   `json:"channelRequired"`
					Value           int    `json:"value"`
				} `json:"setting,omitempty"`
			} `json:"settings"`
			ShowInDataTable  bool   `json:"showInDataTable"`
			StreamToTerminal bool   `json:"streamToTerminal"`
			Type             string `json:"type"`
			NodeID           int    `json:"nodeId"`
		} `json:"analyzers"`
		RowsSettings []struct {
			ID             string `json:"id"`
			Height         int    `json:"height"`
			IsMarkedHidden bool   `json:"isMarkedHidden"`
			Type           string `json:"type"`
			Name           string `json:"name"`
			Channel        struct {
				Category      string `json:"category"`
				Type          string `json:"type"`
				DeviceChannel int    `json:"deviceChannel"`
			} `json:"channel"`
			AnalogScalePerPixel       float64 `json:"analogScalePerPixel,omitempty"`
			AnalogViewportCenterValue int     `json:"analogViewportCenterValue,omitempty"`
		} `json:"rowsSettings"`
		CaptureSettings struct {
			BufferSizeMb           int    `json:"bufferSizeMb"`
			CaptureMode            string `json:"captureMode"`
			StopAfterSeconds       int    `json:"stopAfterSeconds"`
			TrimAfterCapture       bool   `json:"trimAfterCapture"`
			TrimTimeSeconds        int    `json:"trimTimeSeconds"`
			DigitalTriggerSettings struct {
				Type struct {
					Mode    string `json:"mode"`
					Name    string `json:"name"`
					Pattern string `json:"pattern"`
				} `json:"type"`
				TimeAfterTriggerToStop int   `json:"timeAfterTriggerToStop"`
				LinkedChannels         []any `json:"linkedChannels"`
				Duration               struct {
					Min float64 `json:"min"`
					Max float64 `json:"max"`
				} `json:"duration"`
			} `json:"digitalTriggerSettings"`
			GlitchFilter struct {
				Enabled  bool `json:"enabled"`
				Channels []struct {
					Channel struct {
						Category      string `json:"category"`
						Type          string `json:"type"`
						DeviceChannel int    `json:"deviceChannel"`
					} `json:"channel"`
					Filter struct {
						Enabled  bool    `json:"enabled"`
						WidthSec float64 `json:"widthSec"`
					} `json:"filter"`
				} `json:"channels"`
			} `json:"glitchFilter"`
			ConnectedDevice struct {
				Name         string `json:"name"`
				DeviceID     string `json:"deviceId"`
				DeviceType   string `json:"deviceType"`
				IsSimulation bool   `json:"isSimulation"`
				Capabilities struct {
					ChannelCapabilities []struct {
						Type       string `json:"type"`
						Index      int    `json:"index"`
						Capability string `json:"capability"`
					} `json:"channelCapabilities"`
					SampleRateOptions []struct {
						Digital int `json:"digital"`
					} `json:"sampleRateOptions"`
					DigitalThresholdOptions []struct {
						Description string `json:"description"`
					} `json:"digitalThresholdOptions"`
					IsPhysicalDevice bool `json:"isPhysicalDevice"`
				} `json:"capabilities"`
				Settings struct {
					EnabledChannels []struct {
						Type  string `json:"type"`
						Index int    `json:"index"`
					} `json:"enabledChannels"`
					SampleRate struct {
						Digital int `json:"digital"`
					} `json:"sampleRate"`
					DigitalThreshold struct {
						Description string `json:"description"`
					} `json:"digitalThreshold"`
				} `json:"settings"`
			} `json:"connectedDevice"`
		} `json:"captureSettings"`
		DigitalTriggerTime int    `json:"digitalTriggerTime"`
		Name               string `json:"name"`
		DataTable          struct {
			Columns struct {
				AnalyzerIdentifier struct {
					IsActive          bool   `json:"isActive"`
					Width             int    `json:"width"`
					IsDefault         bool   `json:"isDefault"`
					BaseKey           string `json:"baseKey"`
					ExcludeFromSearch bool   `json:"excludeFromSearch"`
				} `json:"analyzerIdentifier"`
				FrameType struct {
					IsActive          bool   `json:"isActive"`
					Width             int    `json:"width"`
					IsDefault         bool   `json:"isDefault"`
					BaseKey           string `json:"baseKey"`
					ExcludeFromSearch bool   `json:"excludeFromSearch"`
				} `json:"frameType"`
				Start struct {
					IsActive          bool   `json:"isActive"`
					Width             int    `json:"width"`
					IsDefault         bool   `json:"isDefault"`
					BaseKey           string `json:"baseKey"`
					ExcludeFromSearch bool   `json:"excludeFromSearch"`
				} `json:"start"`
				Duration struct {
					IsActive          bool   `json:"isActive"`
					Width             int    `json:"width"`
					IsDefault         bool   `json:"isDefault"`
					BaseKey           string `json:"baseKey"`
					ExcludeFromSearch bool   `json:"excludeFromSearch"`
				} `json:"duration"`
				DataMosi struct {
					Width    int    `json:"width"`
					BaseKey  string `json:"baseKey"`
					IsActive bool   `json:"isActive"`
				} `json:"data_mosi"`
				DataMiso struct {
					Width    int    `json:"width"`
					BaseKey  string `json:"baseKey"`
					IsActive bool   `json:"isActive"`
				} `json:"data_miso"`
				DataData struct {
					Width    int    `json:"width"`
					BaseKey  string `json:"baseKey"`
					IsActive bool   `json:"isActive"`
				} `json:"data_data"`
				DataError struct {
					Width    int    `json:"width"`
					BaseKey  string `json:"baseKey"`
					IsActive bool   `json:"isActive"`
				} `json:"data_error"`
				DataAck struct {
					Width    int    `json:"width"`
					BaseKey  string `json:"baseKey"`
					IsActive bool   `json:"isActive"`
				} `json:"data_ack"`
				DataAddress struct {
					Width    int    `json:"width"`
					BaseKey  string `json:"baseKey"`
					IsActive bool   `json:"isActive"`
				} `json:"data_address"`
				DataRead struct {
					Width    int    `json:"width"`
					BaseKey  string `json:"baseKey"`
					IsActive bool   `json:"isActive"`
				} `json:"data_read"`
			} `json:"columns"`
		} `json:"dataTable"`
		AnalyzerTrigger struct {
			Settings struct {
				Enabled        bool    `json:"enabled"`
				SearchQuery    string  `json:"searchQuery"`
				HoldoffSeconds float64 `json:"holdoffSeconds"`
			} `json:"settings"`
		} `json:"analyzerTrigger"`
		TimeManager struct {
			T0 struct {
				Type string `json:"type"`
			} `json:"t0"`
		} `json:"timeManager"`
		CaptureNotes string `json:"captureNotes"`
	} `json:"data"`
	BinData []struct {
		Type  string `json:"type"`
		Index int    `json:"index"`
		File  string `json:"file"`
	} `json:"binData"`
}
