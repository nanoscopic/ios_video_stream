package main

import (
    "fmt"
    "os"
    "os/signal"

    "github.com/nanoscopic/ios_video_stream/screencapture"
    "github.com/nanoscopic/ios_video_stream/screencapture/coremedia"
    "github.com/docopt/docopt-go"
    log "github.com/sirupsen/logrus"
)

func main() {
    usage := fmt.Sprintf(`IOS Video Stream (ios_video_stream) %s

Usage:
  ios_video_stream devices [-v]
  ios_video_stream <zmqpush> <zmqpull> [-v] [--udid=<udid>]`)
    
    arguments, _ := docopt.ParseDoc(usage)
    log.SetFormatter(&log.JSONFormatter{})

    verboseLoggingEnabled, _ := arguments.Bool("-v")
    if verboseLoggingEnabled {
        log.Info("Set Debug mode")
        log.SetLevel(log.DebugLevel)
    }

    devicesCommand, _ := arguments.Bool("devices")
    if devicesCommand { devices(); return }

    udid, _ := arguments.String("--udid")

    streamCommand, _ := arguments.Bool("stream")
    if streamCommand {
        zmqPushSpec, err := arguments.String("<zmqpush>")
        if err != nil {
            log.Errorf("Missing <zmqpush> parameter. Please specify a valid spec like 'tcp://127.0.0.1:7878' - %s", err)
            return
        }
        zmqPullSpec, err := arguments.String("<zmqpull>")
        if err != nil {
            log.Errorf("Missing <zmqpull> parameter. Please specify a valid spec like 'tcp://127.0.0.1:7879' - %s", err)
            return
        }
        stream( zmqPushSpec, zmqPullSpec, udid )
    }
}

func devices() {
    deviceList, err := screencapture.FindIosDevices()
    if err != nil { log.Errorf("Error finding iOS Devices - %s",err) }
    
    for _,device := range deviceList {
        fmt.Printf( "UDID:%s, Name:%s, VID=%s, PID=%s\n", device.SerialNumber, device.ProductName, device.VID, device.PID )
    }
}

func stream( zmqPushSpec string, zmqPullSpec string, udid string) {
    writer := coremedia.NewZMQWriter( zmqPushSpec )
    startWithConsumer( writer, udid )
}

func startWithConsumer( consumer screencapture.CmSampleBufConsumer, udid string )  {
    device, err := screencapture.FindIosDevice(udid)
    if err != nil { log.Errorf("no device found to activate - %s",err); return }

    device, err = screencapture.EnableQTConfig(device)
    if err != nil { log.Errorf("Error enabling QT config - %s",err); return }

    adapter := screencapture.UsbAdapter{}
    stopChannel := make( chan bool )
    waitForSigInt( stopChannel )

    mp := screencapture.NewMessageProcessor(&adapter, stopChannel, consumer)

    err = adapter.StartReading( device, &mp, stopChannel )
    consumer.Stop()
    if err != nil {
        log.Errorf("failed connecting to usb - %s",err)
    }
}

func waitForSigInt( stopChannel chan bool ) {
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)
    go func() {
        for sig := range c {
            log.Debugf("Signal received: %s", sig)
            stopChannel <- true
        }
    }()
}
