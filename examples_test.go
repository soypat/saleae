package saleae_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/soypat/saleae"
	"github.com/soypat/saleae/analyzers"
)

func ExampleReadCaptureFile() {
	startprog := time.Now()
	defer func() {
		fmt.Fprintln(os.Stderr, "elapsed:", time.Since(startprog))
	}()
	cap, err := saleae.ReadCaptureFile("testdata/sx1278_pico.sal")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("capture time:", cap.CaptureStart.Format(time.Stamp))
	for i := range cap.DigitalFiles {
		fmt.Println("digital file version:", cap.DigitalFiles[i].Header.Info.Version)
	}
	//Output:
	//capture time: Jun  4 23:18:12

	// We get an error due to a bug in saleae's software? Getting version==1, expect 0
	// Documentation says current version is 0. What gives?
}

func ExampleDigitalFile_spi() {
	startprog := time.Now()
	defer func() {
		fmt.Fprintln(os.Stderr, "elapsed:", time.Since(startprog))
	}()
	const (
		clockFile  = "testdata/digital_spiclk.bin"
		enableFile = "testdata/digital_spienable.bin"
		sdoFile    = "testdata/digital_spisdo.bin"
		sdiFile    = "testdata/digital_spisdi.bin"
	)
	fp, _ := os.Open(clockFile)
	clock, err := saleae.ReadDigitalFile(fp)
	if err != nil {
		panic(err)
	}
	fp.Close()
	fp, _ = os.Open(enableFile)
	enable, err := saleae.ReadDigitalFile(fp)
	if err != nil {
		panic(err)
	}
	fp.Close()
	fp, _ = os.Open(sdoFile)
	sdo, err := saleae.ReadDigitalFile(fp)
	if err != nil {
		panic(err)
	}
	fp.Close()
	fp, _ = os.Open(sdiFile)
	sdi, err := saleae.ReadDigitalFile(fp)
	if err != nil {
		panic(err)
	}
	fp.Close()
	spi := analyzers.SPI{}
	txs, _ := spi.Scan(clock, enable, sdo, sdi)
	// report, _ := os.Create("report.txt")
	// defer report.Close()
	report := os.Stdout
	var accumulativeResults int
	for i := 0; i < len(txs); i++ {
		tx := txs[i]
		if len(tx.SDO) < 4 {
			panic("too short exchange for cyw43439!")
		}
		cmd, data := CommandFromBytes(tx.SDO)
		for j := i + 1; j < len(txs); j++ {
			accumulativeResults++
			nextcmd, nextdata := CommandFromBytes(txs[j].SDO)
			if nextcmd != cmd || !bytes.Equal(data, nextdata) {
				break
			}
			i = j
		}
		fmt.Fprintf(report, "cmd×%2d %s data=%#x\n", accumulativeResults, cmd.String(), data)
		accumulativeResults = 0
	}
	//Output:
	// cmd× 1 addr=  0x800  fn=      bus  sz=1184 write=false autoinc=false data=0x03030303
	// cmd× 1 addr=  0x800  fn=      bus  sz=1184 write=false autoinc=false data=0xbeadfeed
	// cmd× 1 addr= 0x1800  fn=      bus  sz=1024 write=false autoinc=false data=0x04b30002
	// cmd× 1 addr=    0x0  fn=      bus  sz=   4 write=false autoinc= true data=0xb3000200
	// cmd× 1 addr=   0x1d  fn=      bus  sz=   1 write= true autoinc= true data=0x04000000
	// cmd× 1 addr=    0x4  fn=      bus  sz=   1 write= true autoinc= true data=0x99000000
	// cmd× 1 addr=    0x6  fn=      bus  sz=   2 write= true autoinc= true data=0xbe000000
	// cmd× 1 addr=0x1000e  fn=backplane  sz=   1 write= true autoinc= true data=0x08000000
	// cmd× 1 addr=0x1000e  fn=backplane  sz=   5 write=false autoinc= true data=0x48000040
	// cmd× 1 addr=0x1000e  fn=backplane  sz=   1 write= true autoinc= true data=0x00000000
	// cmd× 1 addr=0x1000c  fn=backplane  sz=   1 write= true autoinc= true data=0x18000000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x10180000
	// cmd× 1 addr= 0x3800  fn=backplane  sz=   5 write=false autoinc= true data=0x01000000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x00180000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x10180000
	// cmd× 1 addr= 0x3800  fn=backplane  sz=   5 write=false autoinc= true data=0x01000000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x00180000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x10180000
	// cmd× 1 addr= 0x4800  fn=backplane  sz=   5 write=false autoinc= true data=0x01000000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x00180000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x10180000
	// cmd× 1 addr= 0x4800  fn=backplane  sz=   5 write=false autoinc= true data=0x01000000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x00180000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x10180000
	// cmd× 1 addr= 0x4800  fn=backplane  sz=   5 write=false autoinc= true data=0x01000000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x00180000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x10180000
	// cmd× 1 addr= 0x4800  fn=backplane  sz=   5 write=false autoinc= true data=0x01000000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x00180000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x10180000
	// cmd× 1 addr= 0x4408  fn=backplane  sz=   1 write= true autoinc= true data=0x03000000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x00180000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x10180000
	// cmd× 1 addr= 0x4408  fn=backplane  sz=   5 write=false autoinc= true data=0x03000000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x00180000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x10180000
	// cmd× 1 addr= 0x4800  fn=backplane  sz=   1 write= true autoinc= true data=0x00000000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x00180000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x10180000
	// cmd× 1 addr= 0x4408  fn=backplane  sz=   1 write= true autoinc= true data=0x01000000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x00180000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x10180000
	// cmd× 1 addr= 0x4408  fn=backplane  sz=   5 write=false autoinc= true data=0x01000000
	// cmd× 1 addr=0x1000b  fn=backplane  sz=   1 write= true autoinc= true data=0x00180000
	// cmd× 1 addr= 0xc010  fn=backplane  sz=   4 write= true autoinc= true data=0x03000000
	// cmd× 1 addr= 0xc044  fn=backplane  sz=   4 write= true autoinc= true data=0x00000000
	// cmd× 0 addr=0x1e00c  fn=backplane  sz=   1 write= true autoinc= true data=0xffffffff
}

type Function uint32

const (
	// All SPI-specific registers.
	FuncBus Function = 0b00
	// Registers and memories belonging to other blocks in the chip (64 bytes max).
	FuncBackplane Function = 0b01
	// DMA channel 1. WLAN packets up to 2048 bytes.
	FuncDMA1 Function = 0b10
	FuncWLAN          = FuncDMA1
	// DMA channel 2 (optional). Packets up to 2048 bytes.
	FuncDMA2 Function = 0b11
)

func (f Function) String() (s string) {
	switch f {
	case FuncBus:
		s = "bus"
	case FuncBackplane:
		s = "backplane"
	case FuncWLAN: // same as FuncDMA1
		s = "wlan"
	case FuncDMA2:
		s = "dma2"
	default:
		s = "unknown"
	}
	return s
}

type CYW43439Cmd struct {
	Write   bool
	AutoInc bool
	Fn      Function
	Addr    uint32
	Size    uint32
}

func (cmd *CYW43439Cmd) String() string {
	return fmt.Sprintf("addr=%#7x  fn=%9s  sz=%4v write=%5v autoinc=%5v",
		cmd.Addr, cmd.Fn.String(), cmd.Size, cmd.Write, cmd.AutoInc)
}

func CommandFromBytes(b []byte) (cmd CYW43439Cmd, data []byte) {
	_ = b[3]
	command := binary.LittleEndian.Uint32(b)
	cmd.Write = command&(1<<31) != 0
	cmd.AutoInc = command&(1<<30) != 0
	cmd.Fn = Function(command>>28) & 0b11
	cmd.Addr = (command >> 11) & 0x1ffff
	cmd.Size = command & ((1 << 11) - 1)
	data = b[4:]
	if cmd.Fn == FuncBackplane && !cmd.Write && len(data) > 4 {
		data = b[8:] // padding.
	}
	return cmd, data
}
