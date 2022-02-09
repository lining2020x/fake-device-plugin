package deviceplugin

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"time"

	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	SocketPath       = pluginapi.DevicePluginPath + "fake-dev.sock"
	FakeResourceName = "demo.org/fake-dev"
	MaxDeviceCounts  = 115 * 1024
)

type FakeDevicePlugin struct {
	resourceName string
	socket       string
	server       *grpc.Server
	stop         chan interface{}
	health       chan interface{}
}

func NewFakeDevicePlugin() *FakeDevicePlugin {
	return &FakeDevicePlugin{
		resourceName: FakeResourceName,
		socket:       SocketPath,
	}
}

func (m *FakeDevicePlugin) Start() error {
	m.server = grpc.NewServer([]grpc.ServerOption{}...)
	m.stop = make(chan interface{})

	err := m.serve()
	if err != nil {
		log.Fatalf("Could not start device plugin for '%s': %s", m.resourceName, err)
		close(m.stop)
		m.server = nil
		m.stop = nil
		return err
	}
	log.Printf("Starting to serve '%s' on %s", m.resourceName, m.socket)

	err = m.register()
	if err != nil {
		log.Fatalf("Could not register device plugin: %s", err)
		m.Stop()
		return err
	}
	log.Printf("Registered device plugin for '%s' with Kubelet", m.resourceName)

	return nil
}

func (m *FakeDevicePlugin) serve() error {
	os.Remove(m.socket)
	sock, err := net.Listen("unix", m.socket)
	if err != nil {
		return err
	}

	pluginapi.RegisterDevicePluginServer(m.server, m)

	go func() {
		log.Printf("Starting GRPC server for '%s', serving on '%s'", m.resourceName, m.socket)
		err := m.server.Serve(sock)
		if err == nil {
			log.Fatalf("GRPC server for '%s' has crashed recently. Quitting", m.resourceName)
		}
	}()

	// Wait for server to start by launching a blocking connection
	// conn, err := m.dial(m.socket, 5*time.Second)
	conn, err := grpc.Dial(m.socket, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))

	if err != nil {
		return err
	}

	conn.Close()

	return nil
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (m *FakeDevicePlugin) register() error {
	// Dial the kubelet socket
	conn, err := grpc.Dial(pluginapi.KubeletSocket, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))

	if err != nil {
		return err
	}

	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	req := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(m.socket),
		ResourceName: m.resourceName,
		Options:      &pluginapi.DevicePluginOptions{},
	}

	_, err = client.Register(context.Background(), req)
	if err != nil {
		return err
	}
	return nil
}

func (m *FakeDevicePlugin) Stop() error {
	if m == nil || m.server == nil {
		return nil
	}

	log.Printf("Stopping to serve '%s' on %s", m.resourceName, m.socket)
	m.server.Stop()
	if err := os.Remove(m.socket); err != nil && !os.IsNotExist(err) {
		return err
	}
	close(m.stop)
	m.server = nil
	m.stop = nil
	return nil
}

// GetDevicePluginOptions returns options to be communicated with Device
// Manager
func (m *FakeDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return nil, nil
}

// ListAndWatch returns a stream of List of Devices
// Whenever a Device state change or a Device disappears, ListAndWatch
// returns the new list
func (m *FakeDevicePlugin) ListAndWatch(req *pluginapi.Empty, srv pluginapi.DevicePlugin_ListAndWatchServer) error {
	fakedevices := make([]*pluginapi.Device, 0)
	for i := 0; i < MaxDeviceCounts; i++ {
		fakedevices = append(fakedevices, &pluginapi.Device{
			ID:     fmt.Sprintf("%s-%s-%d", "vgpu-memory", "1MiB", i),
			Health: pluginapi.Healthy,
		})
	}

	fmt.Printf("devices: %+v\n", len(fakedevices))

	if err := srv.Send(&pluginapi.ListAndWatchResponse{Devices: fakedevices}); err != nil {
		log.Fatalf("Send ListAndWatch Response failed, %v", err)
	}

	for {
		select {
		case <-m.stop:
			return nil
		case <-m.health:
			// some health events handling logic to update fakedevices
			//
			//if err := srv.Send(&pluginapi.ListAndWatchResponse{Devices: fakedevices}); err != nil {
			//	log.Fatalf("Send ListAndWatch Response failed, %v", err)
			//}
		}
	}
}

// GetPreferredAllocation returns a preferred set of devices to allocate
// from a list of available ones. The resulting preferred allocation is not
// guaranteed to be the allocation ultimately performed by the
// devicemanager. It is only designed to help the devicemanager make a more
// informed allocation decision when possible.
func (m *FakeDevicePlugin) GetPreferredAllocation(context.Context, *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return nil, nil
}

// Allocate is called during container creation so that the Device
// Plugin can run device specific operations and instruct Kubelet
// of the steps to make the Device available in the container
func (m *FakeDevicePlugin) Allocate(context.Context, *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	resp := make([]*pluginapi.ContainerAllocateResponse, 0)
	return &pluginapi.AllocateResponse{
		ContainerResponses: resp,
	}, nil
}

// PreStartContainer is called, if indicated by Device Plugin during registeration phase,
// before each container start. Device plugin can run device specific operations
// such as resetting the device before making devices available to the container
func (m *FakeDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return nil, nil
}
