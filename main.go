package main

import (
    "encoding/json"
    "fmt"
    "os"
    "os/signal"

    "github.com/nanoscopic/ios_video_stream/screencapture"
    "github.com/nanoscopic/ios_video_stream/screencapture/coremedia"
    "github.com/docopt/docopt-go"
    log "github.com/sirupsen/logrus"
)

func main() {
    usage := fmt.Sprintf(`Q.uickTime V.ideo H.ack (qvh) %s

Usage:
  qvh devices [-v]
  qvh stream <zmqpush> <zmqpull> [-v] [--udid=<udid>]`)
    
    arguments, _ := docopt.ParseDoc(usage)
    log.SetFormatter(&log.JSONFormatter{})

    verboseLoggingEnabled, _ := arguments.Bool("-v")
    if verboseLoggingEnabled {
        log.Info("Set Debug mode")
        log.SetLevel(log.DebugLevel)
    }

    devicesCommand, _ := arguments.Bool("devices")
    if devicesCommand {
        devices()
        return
    }

    udid, _ := arguments.String("--udid")
    log.Debugf("requested udid:'%s'", udid)

    streamCommand, _ := arguments.Bool("stream")
    if streamCommand {
        zmqPushSpec, err := arguments.String("<zmqpush>")
        if err != nil {
            printErrJSON(err, "Missing <zmqpush> parameter. Please specify a valid spec like 'tcp://127.0.0.1:7878'")
            return
        }
        zmqPullSpec, err := arguments.String("<wzmqpull>")
        if err != nil {
            printErrJSON(err, "Missing <zmqpull> parameter. Please specify a valid spec like 'tcp://127.0.0.1:7878'")
            return
        }
        stream(zmqPushSpec, zmqPullSpec, udid)
    }
}

// Just dump a list of what was discovered to the console
func devices() {
    deviceList, err := screencapture.FindIosDevices()
    if err != nil {
        printErrJSON(err, "Error finding iOS Devices")
    }
    log.Debugf("Found (%d) iOS Devices with UsbMux Endpoint", len(deviceList))

    if err != nil {
        printErrJSON(err, "Error finding iOS Devices")
    }
    output := screencapture.PrintDeviceDetails(deviceList)

    printJSON(map[string]interface{}{"devices": output})
}

func stream( zmqPushSpec string, zmqPullSpec string, udid string) {
    log.Debugf("Writing video output to:'%s'. Receiving jpeg from: %s", zmqPushSpec, zmqPullSpec)

    //writer := coremedia.NewAVFileWriter(bufio.NewWriter(h264File), bufio.NewWriter(wavFile))
    writer := coremedia.NewZMQWriter( zmqPushSpec )

    startWithConsumer( writer, udid )
}

func startWithConsumer( consumer screencapture.CmSampleBufConsumer, udid string )  {
    device, err := screencapture.FindIosDevice(udid)
    if err != nil {
        printErrJSON(err, "no device found to activate")
        return
    }

    device, err = screencapture.EnableQTConfig(device)
    if err != nil {
        printErrJSON(err, "Error enabling QT config")
        return
    }

    adapter := screencapture.UsbAdapter{}
    stopSignal := make(chan interface{})
    waitForSigInt(stopSignal)

    mp := screencapture.NewMessageProcessor(&adapter, stopSignal, consumer)

    err = adapter.StartReading(device, &mp, stopSignal)
    consumer.Stop()
    if err != nil {
        printErrJSON(err, "failed connecting to usb")
    }
}

func waitForSigInt(stopSignalChannel chan interface{}) {
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)
    go func() {
        for sig := range c {
            log.Debugf("Signal received: %s", sig)
            var stopSignal interface{}
            stopSignalChannel <- stopSignal
        }
    }()
}

func printErrJSON(err error, msg string) {
    printJSON(map[string]interface{}{
        "original_error": err.Error(),
        "error_message":    msg,
    })
}
func printJSON(output map[string]interface{}) {
    text, err := json.Marshal(output)
    if err != nil {
        log.Fatalf("Broken json serialization, error: %s", err)
    }
    println(string(text))
}
