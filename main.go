package main

import (
    "flag"
    "fmt"
    "os"
    "os/signal"
    "time"

    "go.nanomsg.org/mangos/v3"
	  "go.nanomsg.org/mangos/v3/protocol/pull"
	  // register transports
	  _ "go.nanomsg.org/mangos/v3/transport/all"
    
	  log "github.com/sirupsen/logrus"
)

func main() {
    var udid       = flag.String( "udid"     , ""                    , "Device UDID" )
    var streamCmd  = flag.Bool(   "stream"   , false                 , "Stream jpegs" )
    var pullSpec   = flag.String( "pullSpec" , "tcp://127.0.0.1:7879", "NanoMsg spec to pull jpeg frames from" )
    var iface      = flag.String( "interface", "none"                , "Network interface to listen on" )
    var port       = flag.String( "port"     , "8000"                , "Network port to listen on" )
    var verbose    = flag.Bool(   "v"        , false                 , "Verbose Debugging" )
    var secure     = flag.Bool( "secure"     , false                 , "Secure HTTPS mode" )
    var cert       = flag.String( "cert"     , "https-server.crt"    , "Cert for HTTP" )
    var key        = flag.String( "key"      , "https-server.key"    , "Key for HTTPS" )
    flag.Parse()
    
    log.SetFormatter(&log.JSONFormatter{})

    if *verbose {
        log.Info("Set Debug mode")
        log.SetLevel(log.DebugLevel)
    }

    if *streamCmd {
        stream( *pullSpec, *udid, *iface, *port, *secure, *cert, *key )
    } else {
        flag.Usage()
    }
}

func stream( pullSpec string, udid string, tunName string, port string, secure bool, cert string, key string ) {
    pullSock := setup_nanomsg_sockets( pullSpec )    
    
    stopChannel := make( chan bool )
    stopChannel2 := make( chan bool )
    waitForSigInt( stopChannel, stopChannel2 )
    
    startJpegServer( pullSock, stopChannel2, port, tunName, secure, cert, key )
        
    <- stopChannel
}

func setup_nanomsg_sockets( pullSpec string ) ( pullSock mangos.Socket ) {
    var err error
        
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
    return pullSock
}

func waitForSigInt( stopChannel chan bool, stopChannel2 chan bool ) {
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)
    go func() {
        for sig := range c {
            fmt.Printf("Got signal %s\n", sig)
            go func() { stopChannel2 <- true }()
            stopChannel <- true
        }
    }()
}
