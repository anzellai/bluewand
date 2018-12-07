package main

//go:generate protoc -I ../bluewand --go_out=plugins=grpc:../bluewand ../bluewand/bluewand.proto

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	pb "github.com/anzellai/bluewand/bluewand"
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/darwin"
	"github.com/go-ble/ble/linux"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	// Wand device name: Kano-Wand-XX-XX-XX
	Wand = "Kano-Wand"
)

// INFO
const (
	// BleUUIDInformationService ...
	BleUUIDInformationService = "64a70010f6914b93a6f40968f5b648f8"
	// BleUUIDInformationOrganisationChar ...
	BleUUIDInformationOrganisationChar = "64a7000bf6914b93a6f40968f5b648f8"
	// BleUUIDInformationSwChar ...
	BleUUIDInformationSwChar = "64a70013f6914b93a6f40968f5b648f8"
	// BleUUIDInformationHwChar ...
	BleUUIDInformationHwChar = "64a70001f6914b93a6f40968f5b648f8"
)

// IO
const (
	// BleUUIDIOService ...
	BleUUIDIOService = "64a70012f6914b93a6f40968f5b648f8"
	// BleUUIDIOBatteryChar ...
	BleUUIDIOBatteryChar = "64a70007f6914b93a6f40968f5b648f8"
	// BleUUIDIOUserButtonChar ...
	BleUUIDIOUserButtonChar = "64a7000df6914b93a6f40968f5b648f8"
	// BleUUIDIOVibratorChar ...
	BleUUIDIOVibratorChar = "64a70008f6914b93a6f40968f5b648f8"
	// BleUUIDIOLedChar ...
	BleUUIDIOLedChar = "64a70009f6914b93a6f40968f5b648f8"
	// BleUUIDIOKeepAliveChar ...
	BleUUIDIOKeepAliveChar = "64a7000ff6914b93a6f40968f5b648f8"
)

// SENSOR
const (
	// BleUUIDSensorService ...
	BleUUIDSensorService = "64a70011f6914b93a6f40968f5b648f8"
	// BleUUIDSensorQuaternionsChar ...
	BleUUIDSensorQuaternionsChar = "64a70002f6914b93a6f40968f5b648f8"
	// BleUUIDSensorRawChar ...
	BleUUIDSensorRawChar = "64a7000af6914b93a6f40968f5b648f8"
	// BleUUIDSensorMotionChar ...
	BleUUIDSensorMotionChar = "64a7000cf6914b93a6f40968f5b648f8"
	// BleUUIDSensorMagnCalibrateChar ...
	BleUUIDSensorMagnCalibrateChar = "64a70021f6914b93a6f40968f5b648f8"
	// BleUUIDSensorQuaternionsResetChar ...
	BleUUIDSensorQuaternionsResetChar = "64a70004f6914b93a6f40968f5b648f8"
	// BleUUIDSensorTempChar ...
	BleUUIDSensorTempChar = "64a70014f6914b93a6f40968f5b648f8"
)

// WandKit struct...
type WandKit struct {
	device        ble.Device
	logger        *log.Entry
	duration      time.Duration
	cln           ble.Client
	p             *ble.Profile
	button        bool
	subscriptions []*ble.Characteristic
}

// New return new instance of WK
func New(l *log.Entry, d time.Duration) *WandKit {
	device, err := NewDevice()
	if err != nil {
		l.Fatalf("can't create new device: %v", err)
	}
	wk := &WandKit{
		device:        device,
		logger:        l,
		duration:      d,
		subscriptions: []*ble.Characteristic{},
	}
	ble.SetDefaultDevice(wk.device)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigc
		if wk.p != nil && wk.subscriptions != nil && len(wk.subscriptions) > 0 {
			for _, subscription := range wk.subscriptions {
				if subscription == nil {
					continue
				}
				subLogger := wk.logger.WithFields(log.Fields{
					"subscription":        subscription.UUID.String(),
					"subscription_name":   ble.Name(subscription.UUID),
					"subscription_handle": fmt.Sprintf("0x%02X", subscription.Handle),
				})
				if err := wk.cln.Unsubscribe(subscription, false); err != nil {
					subLogger.Fatalf("unsubscribe error: %v", err)
				}
				subLogger.Info("subscription unsubscribed")
			}
		}
		defer func() {
			wk.cln = nil
			wk.p = nil
		}()
		err := wk.cln.CancelConnection()
		if err != nil {
			wk.logger.Errorf("can't disconnect: %v", err)
			os.Exit(-1)
		}
		wk.logger.Info("disconnected")
		os.Exit(0)
	}()
	return wk
}

// NewDevice return new Ble Device instance
func NewDevice() (d ble.Device, err error) {
	switch runtime.GOOS {
	case "linux":
		return DefaultLinuxDevice()
	case "windows":
		return nil, errors.New("not implemented")
	default:
		return DefaultDarwinDevice()
	}
}

// DefaultLinuxDevice interface...
func DefaultLinuxDevice() (d ble.Device, err error) {
	return linux.NewDevice()
}

// DefaultDarwinDevice interface...
func DefaultDarwinDevice() (d ble.Device, err error) {
	return darwin.NewDevice()
}

// Connect will scan and get WandKit device
func (wk *WandKit) Connect() {
	filter := func(a ble.Advertisement) bool {
		wk.logger.Infof("scanned device name: %s", a.LocalName())
		return strings.HasPrefix(strings.ToUpper(a.LocalName()), strings.ToUpper(Wand))
	}
	wk.logger.Infof("scanning %s for %s", Wand, wk.duration)
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), wk.duration))
	cln, err := ble.Connect(ctx, filter)
	if err != nil {
		wk.logger.Fatalf("can't connect to %s: %v", Wand, err)
	}
	wk.cln = cln

	wk.logger.Info("discovering profile")
	p, err := cln.DiscoverProfile(true)
	if err != nil {
		wk.logger.Fatalf("can't discover profile: %v", err)
	}
	wk.logger.Infof("profile discovered: %+v", p)
	wk.p = p
}

// Unsubscribe will explore BLE instance and unsubscibe already subscribed characteristic
func (wk *WandKit) Unsubscribe(uid string) {
	for _, c := range wk.subscriptions {
		if c == nil {
			continue
		}
		if c.UUID.Equal(ble.UUID(uid)) {
			chrLogger := wk.logger.WithFields(log.Fields{
				"characteristic":           c.UUID.String(),
				"characteristic_name":      ble.Name(c.UUID),
				"characteristic_property":  propString(c.Property),
				"characteristics_handle":   fmt.Sprintf("0x%02X", c.Handle),
				"characteristics_v_handle": fmt.Sprintf("0x%02X", c.ValueHandle),
			})
			if errUnsubscribe := wk.cln.Unsubscribe(c, true); errUnsubscribe != nil {
				chrLogger.Debugf("unsubscribe error: %v", errUnsubscribe)
			}
			chrLogger.Info("characteristic unsubscribed")
		}
	}
}

// Subscribe will explore BLE instance and subscribe with given characteristic
func (wk *WandKit) Subscribe(uid string, ch chan []byte) {
	wk.logger.Info("connector start")
	for _, s := range wk.p.Services {
		srvLogger := wk.logger.WithFields(log.Fields{
			"service":        s.UUID.String(),
			"service_name":   ble.Name(s.UUID),
			"service_handle": fmt.Sprintf("0x%02X", s.Handle),
		})
		srvLogger.Info("service discovered")

		for _, c := range s.Characteristics {
			chrLogger := srvLogger.WithFields(log.Fields{
				"characteristic":           c.UUID.String(),
				"characteristic_name":      ble.Name(c.UUID),
				"characteristic_property":  propString(c.Property),
				"characteristics_handle":   fmt.Sprintf("0x%02X", c.Handle),
				"characteristics_v_handle": fmt.Sprintf("0x%02X", c.ValueHandle),
			})
			chrLogger.Info("characteristic discovered")

			charUUID := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%s", c.UUID)))
			// We are only interested in given characteristics
			if !(charUUID == uid) {
				continue
			}

			// Don't bother to subscribe the Service Changed characteristics.
			if c.UUID.Equal(ble.ServiceChangedUUID) {
				continue
			}
			// Don't touch the Apple-specific Service/Characteristic.
			// Service: D0611E78BBB44591A5F8487910AE4366
			// Characteristic: 8667556C9A374C9184ED54EE27D90049, Property: 0x18 (WN),
			//   Descriptor: 2902, Client Characteristic Configuration
			//   Value         0000 | "\x00\x00"
			if c.UUID.Equal(ble.MustParse("8667556C9A374C9184ED54EE27D90049")) {
				continue
			}

			if c.Property&ble.CharNotify != 0 {
				chrLogger.Infof("subscribe to notification for %s", wk.duration)
				if err := wk.cln.Subscribe(c, false, func(data []byte) { ch <- data }); err != nil {
					chrLogger.Fatalf("subscribe error: %v", err)
				}
				wk.subscriptions = append(wk.subscriptions, c)
			}

			for _, d := range c.Descriptors {
				dspLogger := chrLogger.WithFields(log.Fields{
					"descriptor":        d.UUID.String(),
					"descriptor_name":   ble.Name(d.UUID),
					"descriptor_handle": fmt.Sprintf("0x%02X", d.Handle),
				})
				dspLogger.Info("descriptor discovered")

				b, err := wk.cln.ReadDescriptor(d)
				if err != nil {
					dspLogger.Errorf("read error: %v", err)
					continue
				}
				dspLogger.Infof("value read: %x | %q", b, b)
			}
		}
	}
}

// ToUint32 helper function to convert 2 bytes to Uint32
func ToUint32(a, b byte) uint32 {
	return uint32(a)<<8 | uint32(b)
}

func propString(p ble.Property) string {
	var s string
	for k, v := range map[ble.Property]string{
		ble.CharBroadcast:   "B",
		ble.CharRead:        "R",
		ble.CharWriteNR:     "w",
		ble.CharWrite:       "W",
		ble.CharNotify:      "N",
		ble.CharIndicate:    "I",
		ble.CharSignedWrite: "S",
		ble.CharExtended:    "E",
	} {
		if p&k != 0 {
			s += v
		}
	}
	return s
}

// Server implementation
//
// This part is to instantiate the gRPC server and all moving parts,
// making use of WandKit instance, abstracting the complexity of handling
// Bluetooth LE protocol.
// So Client can connect and subscribe to callbacks (OnConnect, OnButton, OnMotion)
// very easily to get the callback data from the device.

var (
	// VERSION build var
	VERSION string
	// COMMIT build var
	COMMIT string
	// BRANCH build var
	BRANCH string

	tls  = flag.Bool("tls", false, "Connection to use TLS if set true, otherwise plain TCP is used")
	cert = flag.String("cert", "", "TLS cert file path if TLS is used")
	key  = flag.String("key", "", "TLS key file path if TLS is used")
	port = flag.Int("port", 55555, "gRPC server port to run on")

	// duration is the default context/connection timeout
	duration time.Duration
	// logger is the default logger with Logrus for logging
	logger *log.Entry
	// logLevel to override logging level, default to Info
	logLevel = flag.Int("logLevel", 0, "set log level")
)

// blueWandServer is a struct holding an unique Wand identifier and WandKit instance
type blueWandServer struct {
	identifer *pb.Identifier
	wk        *WandKit
}

// newServer return a new conntected WandKit instance and set up the identifier
func newServer() *blueWandServer {
	s := &blueWandServer{wk: New(logger, duration)}
	s.wk.Connect()
	s.identifer = &pb.Identifier{Uid: s.wk.cln.Addr().String()}
	return s
}

// OnConnect is the callback subscribed to WandKit connection and return Wand Identifier
func (s *blueWandServer) OnConnect(ctx context.Context, empty *pb.EmptyMessage) (*pb.Identifier, error) {
	return s.identifer, nil
}

// OnButton is the callback subscribed to WandKit UserButton characteristics notification callback
func (s *blueWandServer) OnButton(identifier *pb.Identifier, stream pb.BlueWand_OnButtonServer) error {
	if identifier.Uid != s.identifer.Uid {
		return errors.New("mis-matched device identifier")
	}
	ch := make(chan []byte, 1)
	s.wk.Subscribe(BleUUIDIOUserButtonChar, ch)
	for data := range ch {
		s.wk.button = data[0] == 1
		if err := stream.Send(&pb.ButtonMessage{Pressed: s.wk.button}); err != nil {
			s.wk.Unsubscribe(BleUUIDIOUserButtonChar)
			return err
		}
	}
	return nil
}

// OnMotion is the callback subscribed to WandKit Quaternions characteristics notification callback
func (s *blueWandServer) OnMotion(identifier *pb.Identifier, stream pb.BlueWand_OnMotionServer) error {
	if identifier.Uid != s.identifer.Uid {
		return errors.New("mis-matched device identifier")
	}
	ch := make(chan []byte, 1)
	s.wk.Subscribe(BleUUIDSensorQuaternionsChar, ch)
	for data := range ch {
		w := ToUint32(data[0], data[1])
		x := ToUint32(data[2], data[3])
		y := ToUint32(data[4], data[5])
		z := ToUint32(data[6], data[7])
		if err := stream.Send(&pb.MotionMessage{W: w, X: x, Y: y, Z: z}); err != nil {
			s.wk.Unsubscribe(BleUUIDSensorQuaternionsChar)
			return err
		}
	}
	return nil
}

func main() {
	flag.DurationVar(&duration, "duration", time.Duration(time.Second*10), "timeout duration")
	flag.Parse()

	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	switch *logLevel {
	case 1:
		log.SetLevel(log.DebugLevel)
	case 2:
		log.SetLevel(log.WarnLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
	logger = log.WithFields(log.Fields{
		"app":     "WandKit",
		"version": VERSION,
		"commit":  COMMIT[len(COMMIT)-8:],
		"branch":  BRANCH,
	})

	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	if *tls {
		if *cert == "" {
			logger.Fatalln("missing cert file when TLS enabled")
		}
		if *key == "" {
			logger.Fatalln("missing key file when TLS enabled")
		}
		creds, err := credentials.NewServerTLSFromFile(*cert, *key)
		if err != nil {
			logger.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	}
	grpcServer := grpc.NewServer(opts...)
	wandSrv := newServer()
	pb.RegisterBlueWandServer(grpcServer, wandSrv)
	grpcServer.Serve(lis)
}
