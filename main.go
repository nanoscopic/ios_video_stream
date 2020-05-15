package main

import (
    "flag"
    "fmt"
    "os"
    "os/signal"
    "time"

    "github.com/nanoscopic/ios_video_stream/screencapture"
    cm "github.com/nanoscopic/ios_video_stream/screencapture/coremedia"
    
    "go.nanomsg.org/mangos/v3"
	  "go.nanomsg.org/mangos/v3/protocol/pull"
	  "go.nanomsg.org/mangos/v3/protocol/push"
	  // register transports
	  _ "go.nanomsg.org/mangos/v3/transport/all"
    
	  log "github.com/sirupsen/logrus"
)

func main() {
    var udid       = flag.String( "udid"     , ""                    , "Device UDID" )
    var devicesCmd = flag.Bool(   "devices"  , false                 , "List devices then exit" )
    var streamCmd  = flag.Bool(   "stream"   , false                 , "Stream video" )
    var pushSpec   = flag.String( "pushSpec" , "tcp://127.0.0.1:7878", "NanoMsg spec to push h264 nalus to" )
    var pullSpec   = flag.String( "pullSpec" , "tcp://127.0.0.1:7879", "NanoMsg spec to pull jpeg frames from" )
    var iface      = flag.String( "interface", "none"                , "Network interface to listen on" )
    var port       = flag.String( "port"     , "8000"                , "Network port to listen on" )
    var verbose    = flag.Bool(   "v"        , false                 , "Verbose Debugging" )
    flag.Parse()
    
    log.SetFormatter(&log.JSONFormatter{})

    if *verbose {
        log.Info("Set Debug mode")
        log.SetLevel(log.DebugLevel)
    }

    if *devicesCmd {
        devices(); return
    } else if *streamCmd {
        stream( *pushSpec, *pullSpec, *udid, *iface, *port )
    } else {
        flag.Usage()
    }
}

func devices() {
    deviceList, err := screencapture.FindIosDevices()
    if err != nil { log.Errorf("Error finding iOS Devices - %s",err) }
    
    for _,device := range deviceList {
        fmt.Printf( "UDID:%s, Name:%s, VID=%s, PID=%s\n", device.SerialNumber, device.ProductName, device.VID, device.PID )
    }
}

func stream( pushSpec string, pullSpec string, udid string, tunName string, port string ) {
    pushSock, pullSock := setup_nanomsg_sockets( pushSpec, pullSpec )    
    
    stopChannel := make( chan bool )
    stopChannel2 := make( chan bool )
    stopChannel3 := make( chan bool )
    waitForSigInt( stopChannel, stopChannel2, stopChannel3 )
    
    startJpegServer( pullSock, stopChannel2, port, tunName )
    
    writer := cm.NewZMQWriter( pushSock )
    
    attempt := 1
    for {
        success := startWithConsumer( writer, udid, stopChannel )
        if success {
            break
        }
        fmt.Printf("Attempt %i to start streaming\n", attempt)
        if attempt >= 4 {
            log.WithFields( log.Fields{
                "type": "stream_start_failed",
            } ).Fatal("Socket new error")
        }
        attempt++
        time.Sleep( time.Second * 1 )
    }
    
    <- stopChannel3
    writer.Stop()
}

func setup_nanomsg_sockets( pushSpec string, pullSpec string ) ( pushSock mangos.Socket, pullSock mangos.Socket ) {
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
    return pushSock, pullSock
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
