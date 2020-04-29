package main

import (
    "fmt"
    "os"
    "os/signal"
    "time"

    "github.com/nanoscopic/ios_video_stream/screencapture"
    "github.com/nanoscopic/ios_video_stream/screencapture/coremedia"
    "github.com/docopt/docopt-go"
    //zmq "github.com/pebbe/zmq4"
    
    "go.nanomsg.org/mangos/v3"
	  "go.nanomsg.org/mangos/v3/protocol/pull"
	  "go.nanomsg.org/mangos/v3/protocol/push"
	  // register transports
	  _ "go.nanomsg.org/mangos/v3/transport/all"
    
	  log "github.com/sirupsen/logrus"
)

func main() {
    usage := fmt.Sprintf(`IOS Video Stream (ios_video_stream) %s

Usage:
  ios_video_stream devices [-v]
  ios_video_stream stream <pushspec> <pullspec> [-v] [--udid=<udid>]`)
    
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
        pushSpec, err := arguments.String("<pushspec>")
        if err != nil {
            log.Errorf("Missing <pushspec> parameter. Please specify a valid spec like 'tcp://127.0.0.1:7878' - %s", err)
            return
        }
        pullSpec, err := arguments.String("<pullspec>")
        if err != nil {
            log.Errorf("Missing <pullspec> parameter. Please specify a valid spec like 'tcp://127.0.0.1:7879' - %s", err)
            return
        }
        stream( pushSpec, pullSpec, udid )
    }
}

func devices() {
    deviceList, err := screencapture.FindIosDevices()
    if err != nil { log.Errorf("Error finding iOS Devices - %s",err) }
    
    for _,device := range deviceList {
        fmt.Printf( "UDID:%s, Name:%s, VID=%s, PID=%s\n", device.SerialNumber, device.ProductName, device.VID, device.PID )
    }
}

func stream( pushSpec string, pullSpec string, udid string ) {
    /*pushSock, _ := zmq.NewSocket(zmq.PUSH)
    err := pushSock.Connect( zmqPushSpec )
    if err != nil {
        log.WithFields( log.Fields{
            "type": "err_zmq_connect",
            "zmq_spec": zmqPushSpec,
            "err": err,
        } ).Fatal("ZMQ connect error")
    }*/
    
    var pushSock mangos.Socket
    var err error
    if pushSock, err = push.NewSocket(); err != nil {
        log.WithFields( log.Fields{
            "type": "err_socket_new",
            "spec": pushSpec,
            "err": err,
        } ).Fatal("Socket new error")
    }
    if err = pushSock.Dial(pushSpec); err != nil {
        log.WithFields( log.Fields{
            "type": "err_socket_connect",
            "spec": pushSpec,
            "err": err,
        } ).Fatal("Socket connect error")
    }
    
    // Garbage message with delay to avoid late joiner ZeroMQ madness
    //msg := "dummy"
    //pushSock.Send(msg,0)
    //pushSock.Send( []byte (msg) )
    //time.Sleep( time.Millisecond * 300 )
    
    /*pullSock, _ := zmq.NewSocket(zmq.PULL)
    err = pullSock.Bind( zmqPullSpec )
    if err != nil {
        log.WithFields( log.Fields{
            "type": "err_zmq_bind",
            "zmq_spec": zmqPullSpec,
            "err": err,
        } ).Fatal("ZMQ bind error")
    }*/
    var pullSock mangos.Socket
    if pullSock, err = pull.NewSocket(); err != nil {
        log.WithFields( log.Fields{
            "type": "err_socket_new",
            "zmq_spec": pullSpec,
            "err": err,
        } ).Fatal("Socket new error")
    }
    pullSock.SetOption( mangos.OptionRecvDeadline, time.Duration.Seconds( 1 ) )
    if err = pullSock.Listen(pullSpec); err != nil {
        log.WithFields( log.Fields{
            "type": "err_socket_bind",
            "spec": pullSpec,
            "err": err,
        } ).Fatal("Socket bind error")
    }
    
    stopChannel := make( chan bool )
    stopChannel2 := make( chan bool )
    stopChannel3 := make( chan bool )
    waitForSigInt( stopChannel, stopChannel2, stopChannel3 )
    
    startJpegServer( pullSock, stopChannel2, "none" )
    
    writer := coremedia.NewZMQWriter( pushSock )
    success := startWithConsumer( writer, udid, stopChannel )
    
    if success == true {    
        <- stopChannel3
        writer.Stop()
    }
}

func startWithConsumer( consumer screencapture.CmSampleBufConsumer, udid string, stopChannel chan bool )  ( bool ) {
    device, err := screencapture.FindIosDevice(udid)
    if err != nil { log.Errorf("no device found to activate - %s",err); return false }

    device, err = screencapture.EnableQTConfig(device)
    if err != nil { log.Errorf("Error enabling QT config - %s",err); return false }

    adapter := screencapture.UsbAdapter{}
 
    mp := screencapture.NewMessageProcessor( &adapter, stopChannel, consumer )

    err = adapter.StartReading( device, &mp, stopChannel )
    
    if err != nil { log.Errorf("failed connecting to usb - %s",err); return false }
    
    return true
}

func waitForSigInt( stopChannel chan bool, stopChannel2 chan bool, stopChannel3 chan bool ) {
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)
    go func() {
        for sig := range c {
            fmt.Printf("Got signal %s\n", sig)
            //log.Debugf("Signal received: %s", sig)
            go func() { stopChannel2 <- true }()
            stopChannel3 <- true
            stopChannel <- true
            
        }
    }()
}
