package msd347

import (
	"github.com/kylelemons/gousb/usb"
)

type conn struct {
	ctx    *usb.Context
	device *usb.Device

	input  usb.Endpoint
	output usb.Endpoint
}

func (p *conn) Close() error {
	if err := p.device.Close(); err != nil {
		return err
	}
	if err := p.ctx.Close(); err != nil {
		return err
	}

	return nil
}

func (p *conn) Read(buf []byte) (int, error) {
	return p.output.Read(buf)
}

func (p *conn) Write(buf []byte) (int, error) {
	return p.input.Write(buf)
}

func connectUSB() (*conn, error) {
	ctx := usb.NewContext()

	dev, err := ctx.OpenDeviceWithVidPid(0x0519, 0x2013)
	if err != nil {
		return nil, err
	}

	in, err := dev.OpenEndpoint(1, 0, 0, 3)
	if err != nil {
		return nil, err
	}

	out, err := dev.OpenEndpoint(1, 0, 0, 129)
	if err != nil {
		return nil, err
	}

	return &conn{
		device: dev,
		ctx:    ctx,
		input:  in,
		output: out,
	}, nil
}
