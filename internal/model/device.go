package model

// Device is an attached Android device known to adb.
type Device struct {
	Serial    string `json:"serial"`    // adb serial (USB serial or ip:port)
	State     string `json:"state"`     // device | offline | unauthorized
	Model     string `json:"model"`     // human model name
	Transport string `json:"transport"` // usb | tcpip
}
