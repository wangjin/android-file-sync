package main

import (
	"strings"

	"androidfs/internal/model"
)

// GetDevices returns currently attached devices.
func (a *App) GetDevices() []model.Device {
	devs, err := a.client.ListDevices(a.ctx)
	if err != nil {
		return []model.Device{}
	}
	return devs
}

// ConnectDevice attaches a wireless device by ip:port.
func (a *App) ConnectDevice(addr string) (string, error) {
	return a.client.Connect(a.ctx, addr)
}

func deviceSignature(devs []model.Device) string {
	var b strings.Builder
	for _, d := range devs {
		b.WriteString(d.Serial)
		b.WriteString("|")
		b.WriteString(d.State)
		b.WriteString(";")
	}
	return b.String()
}
