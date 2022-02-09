package main

import (
	"github.com/lining2020x/fake-device-plugin/pkg/deviceplugin"
)

func main() {
	plugin := deviceplugin.NewFakeDevicePlugin()
	plugin.Start()

	ch := make(chan interface{}, 1)
	<-ch
}
