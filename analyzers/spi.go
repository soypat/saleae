package analyzers

import (
	"math"

	"github.com/soypat/saleae"
)

type TxSPI struct {
	// a.k.a. MOSI.
	SDO []byte
	// a.k.a. MISO.
	SDI     []byte
	timings []Interval
}

func (t TxSPI) StartTime() float64 {
	if len(t.timings) < 1 {
		return math.NaN()
	}
	return t.timings[0].start
}

func (t TxSPI) EndTime() float64 {
	if len(t.timings) < 1 {
		return math.NaN()
	}
	return t.timings[len(t.timings)-1].end
}

type Interval struct {
	start float64
	end   float64
}

// SPI can be used to analyze a digital signal for SPI transactions. For now
// only supports MODE 0, MSB first, 8 bits per transfer, enable line active low.
type SPI struct {
}

func (*SPI) Scan(clock, enable, mosi, miso *saleae.DigitalFile) (txs []TxSPI, err error) {
	clkState := clock.Header.InitialState != 0
	mosiState := mosi.Header.InitialState != 0
	misoState := miso.Header.InitialState != 0
	enableState := enable.Header.InitialState != 0

	var (
		timeStartForByte                 float64
		currentMisoByte, currentMosiByte byte
		misoBytes, mosiBytes             []byte
		timings                          []Interval
		startByteIdx, bitIdx             int
	)
	iclk := 0
	if clkState {
		iclk = 1 // Only iterate over rising flanks.
	}
	mosiLast := 0
	misoLast := 0
	enableLast := 0
	tMosi := mosi.Data[mosiLast]
	tMiso := miso.Data[misoLast]
	tEnable := enable.Data[enableLast]
	for ; iclk < len(clock.Data); iclk += 2 {
		t := clock.Data[iclk]
		for t > tEnable && enableLast < len(enable.Data)-1 {
			enableLast++
			tEnable = enable.Data[enableLast]
			enableState = !enableState
			if enableState && len(misoBytes[startByteIdx:]) > 0 {
				txs = append(txs, TxSPI{
					SDI:     misoBytes[startByteIdx:],
					SDO:     mosiBytes[startByteIdx:],
					timings: timings[startByteIdx:],
				})
				startByteIdx = len(misoBytes)
				currentMisoByte = 0
				bitIdx = 0
			}
		}
		if enableState {
			continue
		}
		for t > tMiso && misoLast < len(miso.Data)-1 {
			misoLast++
			tMiso = miso.Data[misoLast]
			misoState = !misoState
		}
		for t > tMosi && mosiLast < len(mosi.Data)-1 {
			mosiLast++
			tMosi = mosi.Data[mosiLast]
			mosiState = !mosiState
		}
		if bitIdx == 0 {
			timeStartForByte = t
		}
		currentMisoByte |= b2u8(misoState) << (7 - byte(bitIdx))
		currentMosiByte |= b2u8(mosiState) << (7 - byte(bitIdx))

		bitIdx++
		if bitIdx%8 == 0 {
			timings = append(timings, Interval{start: timeStartForByte, end: t})
			misoBytes = append(misoBytes, currentMisoByte)
			mosiBytes = append(mosiBytes, currentMosiByte)
			currentMisoByte = 0
			currentMosiByte = 0
			bitIdx = 0
		}
	}
	if len(misoBytes[startByteIdx:]) > 0 {
		txs = append(txs, TxSPI{
			SDI:     misoBytes[startByteIdx:],
			SDO:     mosiBytes[startByteIdx:],
			timings: timings[startByteIdx:],
		})
	}
	return txs, nil
}

func b2u8(b bool) byte {
	if b {
		return 1
	}
	return 0
}
