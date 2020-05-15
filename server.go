package main

import (
    //"bytes"
    "fmt"
    "html/template"
    //"image"
    _ "image/jpeg"
    //"io"
    "log"
    "net"
    "net/http"
    "os"
    "sync"
    "time"
    
    "github.com/gorilla/websocket"
    
    "go.nanomsg.org/mangos/v3"
	  // register transports
	  _ "go.nanomsg.org/mangos/v3/transport/all"
)

func callback( r *http.Request ) bool {
    return true
}

var upgrader = websocket.Upgrader {
    CheckOrigin: callback,
}

type ImgMsg struct {
    imgNum int
    msg string
    data []byte
}

type ImgType struct {
    rawData []byte
}

const (
    WriterStop = iota
)

type WriterMsg struct {
    msg int
}

const (
    DummyStart = iota
    DummyStop
)

type DummyMsg struct {
    msg int
}

type Stats struct {
    recv int
    dumped int
    sent int
    dummyActive bool
    socketConnected bool
    waitCnt int
}

func startJpegServer( inSock mangos.Socket, stopChannel chan bool, mirrorPort string, tunName string ) {
    var err error
    
    ifaces, err := net.Interfaces()
    if err != nil {
        fmt.Printf( err.Error() )
        os.Exit( 1 )
    }
    
    var listen_addr string
    if tunName == "none" {
        fmt.Printf("No tunnel specified; listening on all interfaces\n")
        listen_addr = ":" + mirrorPort
    } else {
        foundInterface := false
        for _, iface := range ifaces {
            addrs, err := iface.Addrs()
            if err != nil {
                fmt.Printf( err.Error() )
                os.Exit( 1 )
            }
            for _, addr := range addrs {
                var ip net.IP
                switch v := addr.(type) {
                    case *net.IPNet:
                        ip = v.IP
                    case *net.IPAddr:
                        ip = v.IP
                    default:
                        fmt.Printf("Unknown type\n")
                }
                if iface.Name == tunName {
                    listen_addr = ip.String() + ":" + mirrorPort
                    foundInterface = true
                }
            }
        }
        if foundInterface == false {
            fmt.Printf( "Could not find interface %s\n", tunName )
            os.Exit( 1 )
        }
    }
  
    imgCh := make(chan ImgMsg, 60)
    dummyCh := make(chan DummyMsg, 2)
    
    lock := sync.RWMutex{}
    var stats Stats = Stats{}
    stats.recv = 0
    stats.dumped = 0
    stats.dummyActive = false
    stats.socketConnected = false
    
    statLock := sync.RWMutex{}
    var dummyRunning = true
    
    startDummyReceiver( imgCh, dummyCh, &lock, &statLock, &stats, &dummyRunning )
    
    go func() {
        imgnum := 1
        
        LOOP:
        for {
            select {
                case <- stopChannel:
                    fmt.Printf("Server channel got stop message\n")
                    break LOOP
                default:
            }
            
            msg, err := inSock.RecvMsg()

            if err != nil && err ==  mangos.ErrRecvTimeout {
                continue
            }
            
            //reader := bytes.NewReader( msg.Body )
            //config, _, _ := image.DecodeConfig( reader ) // ( config, format, error )
            //msg := fmt.Sprintf("Width: %d, Height: %d, Size: %d\n", config.Width, config.Height, len( jpegStr ) )
            
            discard := false
            lock.Lock()
            if dummyRunning { discard = true }
            lock.Unlock()
            if !discard {
                imgMsg := ImgMsg{}
                imgMsg.imgNum = imgnum
                imgMsg.data = []byte ( msg.Body )
                imgMsg.msg = "none"
                imgCh <- imgMsg
                msg.Free()
            } else {
                msg.Free()
            }
            
            statLock.Lock()
            stats.recv++
            statLock.Unlock()

            imgnum++
        }
    }()
    
    startServer( imgCh, dummyCh, &lock, &statLock, &stats, &dummyRunning, listen_addr )
}

func startDummyReceiver(imgCh <-chan ImgMsg,dummyCh <-chan DummyMsg,lock *sync.RWMutex, statLock *sync.RWMutex, stats *Stats, dummyRunning *bool ) {
    go func() {
        statLock.Lock()
        stats.dummyActive = true
        statLock.Unlock()
        var running bool = true
        lock.Lock()
        *dummyRunning = true
        lock.Unlock()
        for {
            select {
                  case <- imgCh:
                      // dump the image
                      statLock.Lock()
                      stats.dumped++
                      statLock.Unlock()
                  case controlMsg := <- dummyCh:
                      if controlMsg.msg == DummyStop {
                          running = false
                          lock.Lock()
                          *dummyRunning = false
                          lock.Unlock()
                          statLock.Lock()
                          stats.dummyActive = false
                          statLock.Unlock()
                      }
            }
            if running == false {
                break
            }
        }
    }()
}

func startWriter(ws *websocket.Conn,imgCh <-chan ImgMsg,writerCh <-chan WriterMsg,lock *sync.RWMutex, statLock *sync.RWMutex, stats *Stats) {
    go func() {
        var running bool = true
        statLock.Lock()
        stats.socketConnected = false
        statLock.Unlock()
        for {
            select {
                case imgMsg := <- imgCh:
                    // Keep receiving images till there are no more to receive
                    stop := false
                    for {
                        select {
                        case imgMsg = <- imgCh:
                            
                        default:
                            stop = true
                        }
                        if stop { break }
                    }
            
                    msg := []byte(imgMsg.msg)
                    imgBytes := imgMsg.data
                    
                    ws.WriteMessage(websocket.TextMessage, msg)
                    ws.WriteMessage(websocket.BinaryMessage, imgBytes )
                case controlMsg := <- writerCh:
                    if controlMsg.msg == WriterStop {
                        running = false
                    }
            }
            if running == false {
                break
            }
        }
        statLock.Lock()
        stats.socketConnected = false
        statLock.Unlock()
    }()
}

func startServer( imgCh <-chan ImgMsg, dummyCh chan DummyMsg, lock *sync.RWMutex, statLock *sync.RWMutex, stats *Stats, dummyRunning *bool, listen_addr string ) (*http.Server) {
    fmt.Printf("Listening on %s\n", listen_addr )
    
    echoClosure := func( w http.ResponseWriter, r *http.Request ) {
        handleEcho( w, r, imgCh, dummyCh, lock, statLock, stats, dummyRunning )
    }
    statsClosure := func( w http.ResponseWriter, r *http.Request ) {
        handleStats( w, r, statLock, stats )
    }
    
    server := &http.Server{ Addr: listen_addr }
    http.HandleFunc( "/echo", echoClosure )
    http.HandleFunc( "/echo/", echoClosure )
    http.HandleFunc( "/", handleRoot )
    http.HandleFunc( "/stats", statsClosure )
    go func() {
        server.ListenAndServe()
    }()
    return server
}

func handleStats( w http.ResponseWriter, r *http.Request, statLock *sync.RWMutex, stats *Stats ) {
    statLock.Lock()
    recv := stats.recv
    dumped := stats.dumped
    dummyActive := stats.dummyActive
    socketConnected := stats.socketConnected
    statLock.Unlock()
    waitCnt := stats.waitCnt
    
    var dummyStr string = "no"
    if dummyActive {
        dummyStr = "yes"
    }
    
    var socketStr string = "no"
    if socketConnected {
        socketStr = "yes"
    }
    
    fmt.Fprintf( w, "Received: %d<br>\nDumped: %d<br>\nDummy Active: %s<br>\nSocket Connected: %s<br>\nWait Count: %d<br>\n", recv, dumped, dummyStr, socketStr, waitCnt )
}

func handleEcho( w http.ResponseWriter, r *http.Request,imgCh <-chan ImgMsg,dummyCh chan DummyMsg,lock *sync.RWMutex, statLock *sync.RWMutex, stats *Stats, dummyRunning *bool) {
    c, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Print("Upgrade error:", err)
        return
    }
    defer c.Close()
    fmt.Printf("Received connection\n")
    welcome(c)
        
    writerCh := make(chan WriterMsg, 2)
    
    // stop Dummy Reader from sucking up images
    stopMsg1 := DummyMsg{}
    stopMsg1.msg = DummyStop
    dummyCh <- stopMsg1
    
    // ensure the Dummy Reader has stopped
    for {
        lock.Lock()
        if !*dummyRunning {
            lock.Unlock()
            break
        }
        lock.Unlock()
        time.Sleep( time.Second * 1 )
        statLock.Lock()
        stats.waitCnt++
        statLock.Unlock()
    }
    
    stopped := false
    
    c.SetCloseHandler( func( code int, text string ) error {
        stopped = true
        stopMsg := WriterMsg{}
        stopMsg.msg = WriterStop
        writerCh <- stopMsg
        return nil
    } )
    
    startWriter(c,imgCh,writerCh,lock,statLock,stats)
    for {
        mt, message, err := c.ReadMessage()
        if err != nil {
            log.Println("read:", err)
            break
        }
        log.Printf("recv: %s", message)
        lock.Lock()
        err = c.WriteMessage(mt, message)
        lock.Unlock()
        if err != nil {
            log.Println("write:", err)
            break
        }
    }
    
    // send WriterMsg to terminate writer
    if !stopped {
        stopMsg := WriterMsg{}
        stopMsg.msg = WriterStop
        writerCh <- stopMsg
    }
    
    startDummyReceiver( imgCh, dummyCh, lock, statLock, stats, dummyRunning )
}

func welcome( c *websocket.Conn ) ( error ) {
    msg := `
{
    "version":1,
    "length":24,
    "pid":12733,
    "realWidth":750,
    "realHeight":1334,
    "virtualWidth":375,
    "virtualHeight":667,
    "orientation":0,
    "quirks":{
        "dumb":false,
        "alwaysUpright":true,
        "tear":false
    }
}`
    return c.WriteMessage( websocket.TextMessage, []byte(msg) )
}

func handleRoot( w http.ResponseWriter, r *http.Request ) {
    rootTpl.Execute( w, "ws://"+r.Host+"/echo" )
}

var rootTpl = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  canvas {
    border: solid 1px black;
  }
</style>
<script>
  function getel( id ) {
    return document.getElementById( id );
  }
  
  window.addEventListener("load", function(evt) {
    var output = getel("output");
    var input  = getel("input");
    var ctx    = getel("canvas").getContext("2d");
    var ws;
    
    getel("open").onclick = function( event ) {
      if( ws ) {
        return false;
      }
      ws = new WebSocket("{{.}}");
      ws.onopen = function( event ) {
        console.log("Websocket open");
      }
      ws.onclose = function( event ) {
        console.log("Websocket closed");
        ws = null;
      }
      ws.onmessage = function( event ) {
        if( event.data instanceof Blob ) {
          var image = new Image();
          var url;
          image.onload = function() {
            ctx.drawImage(image, 0, 0);
            URL.revokeObjectURL( url );
          };
          image.onerror = function( e ) {
            console.log('Error during loading image:', e);
          }
          var blob = event.data;
          
          url = URL.createObjectURL( blob );
          image.src = url;
        }
        else {
          //var text = "Response: " + event.data;
          //var d = document.createElement("div");
          //d.innerHTML = text;
          //output.appendChild( d );
        }
      }
      ws.onerror = function( event ) {
        console.log( "Error: ", event.data );
      }
      return false;
    };
    getel("send").onclick = function( event ) {
      if( !ws ) return false;
      ws.send( input.value );
      return false;
    };
    getel("close").onclick = function( event)  {
      if(!ws) return false;
      ws.close();
      return false;
    };
  });
</script>
</head>
<body>
  <table>
    <tr>
      <td valign="top">
        <canvas id="canvas" width="750" height="1334"></canvas>
      </td>
      <td valign="top" width="50%">
        <form>
          <button id="open">Open</button>
          <button id="close">Close</button>
          <br>
          <input id="input" type="text" value="">
          <button id="send">Send</button>
        </form>
      </td>
      <td valign="top" width="50%">
        <div id="output"></div>
      </td>
    </tr>
  </table>
</body>
</html>
`))