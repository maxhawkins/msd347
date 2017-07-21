package msd347

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"sync"
)

const (
	ESC byte = 27
	GS  byte = 29
	SP  byte = 32
	DLE byte = 16
	EOT byte = 4
)

type Justification byte

const (
	JustifyLeft   Justification = 0
	JustifyCenter Justification = 1
	JustifyRight  Justification = 2
)

const (
	Status0 = 1 << iota
	Status2
	TicketTaken
)

const (
	ErrorStatus0 = 1 << iota
	ErrorStatus1
	ErrorStatusMechanical
	ErrorStatusAutocutter
	ErrorStatus4
	ErrorStatusUnrecoverable
	ErrorStatusAutorecoverable
)

type PrintMode byte

const (
	PrintNormal       PrintMode = 0
	PrintDoubleHeight PrintMode = 1
	PrintDoubleWidth  PrintMode = 2
	PrintQuadruple    PrintMode = 3
)

type ErrorInfo struct {
	MechanicalError      bool
	AutocutterError      bool
	UnrecoverableError   bool
	AutorecoverableError bool
}

type TicketInfo struct {
	TicketTaken bool
}

func Connect() (*Printer, error) {
	conn, err := connectUSB()
	if err != nil {
		return nil, err
	}

	return &Printer{
		conn: conn,
	}, nil
}

type Printer struct {
	conn io.ReadWriteCloser

	statusMu sync.Mutex
}

func (p *Printer) Close() error {
	return p.conn.Close()
}

func (p *Printer) Initialize() error {
	cmd := []byte{ESC, '@'}

	if _, err := p.conn.Write(cmd); err != nil {
		return err
	}

	return nil
}

func (p *Printer) SetButtonsEnabled(enabled bool) error {
	n := byte(1)
	if enabled {
		n = 0
	}

	cmd := []byte{ESC, 'c', '5', n}
	if _, err := p.conn.Write(cmd); err != nil {
		return err
	}

	return nil
}

func (p *Printer) PrintImage(img image.Image, m PrintMode) error {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	// TODO(maxhawkins): deal with non power of two textures
	bytesW := width / 8
	bytesH := height

	bwImg := image.NewPaletted(img.Bounds(), []color.Color{color.White, color.Black})
	draw.FloydSteinberg.Draw(bwImg, img.Bounds(), img, image.ZP)
	// draw.Draw(bwImg, img.Bounds(), img, image.ZP, draw.Over)

	if bytesW > 128 {
		return fmt.Errorf("width %d > max 128 bytes (1024 dots)", bytesW)
	}
	if bytesH > 4095 {
		return fmt.Errorf("height %d > max 4095 bytes (4095 dots)", bytesH)
	}

	widthL := byte(bytesW & 0xFF)
	widthH := byte((bytesW >> 8) & 0xFF)

	heightL := byte(bytesH & 0xFF)
	heightH := byte((bytesH >> 8) & 0xFF)

	data := make([]byte, bytesW*bytesH)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			i := x/8 + bytesW*y
			mask := uint8(1) << uint8(7-x%8)

			px := bwImg.ColorIndexAt(x, y)
			if px > 0 {
				data[i] |= mask
			}
		}
	}

	cmd := []byte{GS, 'v', '0', byte(m), widthL, widthH, heightL, heightH}
	cmd = append(cmd, data...)
	buf := bytes.NewBuffer(cmd)

	for {
		_, err := io.CopyN(p.conn, buf, 64)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Printer) SetJustification(j Justification) error {
	cmd := []byte{ESC, 'a', byte(j)}

	if _, err := p.conn.Write(cmd); err != nil {
		return err
	}

	return nil
}

func (p *Printer) FullCut() error {
	cmd := []byte{ESC, 'i'}

	if _, err := p.conn.Write(cmd); err != nil {
		return err
	}

	return nil
}

func (e ErrorInfo) Error() string {
	var s string

	if e.MechanicalError {
		s = "mechanical error"
	} else if e.AutocutterError {
		s = "autocutter error"
	} else {
		s = "printer error"
	}

	if e.UnrecoverableError {
		s += " (unrecoverable)"
	} else if e.AutorecoverableError {
		s += " (recoverable)"
	}

	return s
}

func (e ErrorInfo) OK() bool {
	return !(e.MechanicalError ||
		e.AutocutterError ||
		e.UnrecoverableError ||
		e.AutocutterError)
}

func (p *Printer) QueryErr() error {
	p.statusMu.Lock()
	defer p.statusMu.Unlock()

	cmd := []byte{DLE, EOT, 3}
	if _, err := p.conn.Write(cmd); err != nil {
		return err
	}

	buf := make([]byte, 1)
	i, err := p.conn.Read(buf)
	if err != nil {
		return err
	}
	if i < 1 {
		return errors.New("query error: expected to read byte")
	}
	val := buf[0]

	info := &ErrorInfo{
		MechanicalError:      (val&ErrorStatusMechanical == 1),
		AutocutterError:      (val&ErrorStatusAutocutter == 1),
		UnrecoverableError:   (val&ErrorStatusUnrecoverable == 1),
		AutorecoverableError: (val&ErrorStatusAutorecoverable == 1),
	}
	if info.OK() {
		return nil
	}

	return info
}

func (p *Printer) GetTicketInfo() (TicketInfo, error) {
	var info TicketInfo

	p.statusMu.Lock()
	defer p.statusMu.Unlock()

	cmd := []byte{DLE, EOT, 5}
	if _, err := p.conn.Write(cmd); err != nil {
		return info, err
	}

	buf := make([]byte, 1)
	i, err := p.conn.Read(buf)
	if err != nil {
		return info, err
	}
	if i < 1 {
		return info, errors.New("ticket info: expected to read byte")
	}
	val := buf[0]

	info.TicketTaken = (val&TicketTaken == 0)

	return info, nil
}
