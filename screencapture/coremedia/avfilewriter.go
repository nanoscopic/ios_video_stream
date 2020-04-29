package coremedia

import (
    "bytes"
    "encoding/binary"
    //"fmt"
    //zmq "github.com/pebbe/zmq4"
    
    "go.nanomsg.org/mangos/v3"
	  //"go.nanomsg.org/mangos/v3/protocol/pull"
	  //"go.nanomsg.org/mangos/v3/protocol/push"
	  // register transports
	  _ "go.nanomsg.org/mangos/v3/transport/all"
)

var startCode = []byte{00, 00, 00, 01}

//ZMQWriter writes nalus into a file using 0x00000001 as a separator (h264 ANNEX B) and raw pcm audio into a wav file
type ZMQWriter struct {
    //socket        *zmq.Socket
    socket       mangos.Socket
    buffer       bytes.Buffer
    outFilePath  string
}

//NewZMQWriter binary writes nalus in annex b format to the given writer and audio buffers into a wav file.
//func NewZMQWriter( socket *zmq.Socket ) ZMQWriter {
func NewZMQWriter( socket mangos.Socket ) ZMQWriter {
    return ZMQWriter{ socket: socket }
}

//Consume writes PPS and SPS as well as sample bufs into a annex b .h264 file and audio samples into a wav file
func (avfw ZMQWriter) Consume(buf CMSampleBuffer) error {
    if buf.MediaType == MediaTypeSound {
        return avfw.consumeAudio(buf)
    }
    return avfw.consumeVideo(buf)
}

func (self ZMQWriter) Stop() {}

func (self ZMQWriter) consumeVideo(buf CMSampleBuffer) error {
    if buf.HasFormatDescription {
        err := self.writeNalu(buf.FormatDescription.PPS)
        if err != nil {
            return err
        }
        err = self.writeNalu(buf.FormatDescription.SPS)
        if err != nil {
            return err
        }
    }
    if !buf.HasSampleData() {
        return nil
    }
    return self.writeNalus(buf.SampleData)
}

func (self ZMQWriter) writeNalus(bytes []byte) error {
    slice := bytes
    for len(slice) > 0 {
        length := binary.BigEndian.Uint32(slice)
        err := self.writeNalu(slice[4 : length+4])
        if err != nil {
            return err
        }
        slice = slice[length+4:]
    }
    return nil
}

func (self ZMQWriter) writeNalu(naluBytes []byte) error {
    _, err := self.buffer.Write(startCode)
    if err != nil {
        return err
    }
    _, err = self.buffer.Write(naluBytes)
    if err != nil {
        return err
    }
    //fmt.Printf(".\n")
    //self.socket.Send( self.buffer.String(), 0 )
    self.socket.Send( self.buffer.Bytes() )
    self.buffer.Reset()
    return nil
}

func (self ZMQWriter) consumeAudio(buffer CMSampleBuffer) error {
    return nil
}
