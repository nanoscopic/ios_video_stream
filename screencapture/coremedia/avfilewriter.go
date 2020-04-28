package coremedia

import (
    "encoding/binary"
    "io"
)

var startCode = []byte{00, 00, 00, 01}

//ZMQWriter writes nalus into a file using 0x00000001 as a separator (h264 ANNEX B) and raw pcm audio into a wav file
type ZMQWriter struct {
    zmqPushSpec  string
    bufferWriter io.Writer
    outFilePath  string
}

//NewZMQWriter binary writes nalus in annex b format to the given writer and audio buffers into a wav file.
func NewZMQWriter( zmqPushSpec string ) ZMQWriter {
    return ZMQWriter{ zmqPushSpec: zmqPushSpec }
}

//Consume writes PPS and SPS as well as sample bufs into a annex b .h264 file and audio samples into a wav file
func (avfw ZMQWriter) Consume(buf CMSampleBuffer) error {
    if buf.MediaType == MediaTypeSound {
        return avfw.consumeAudio(buf)
    }
    return avfw.consumeVideo(buf)
}

func (avfw ZMQWriter) Stop() {}

func (avfw ZMQWriter) consumeVideo(buf CMSampleBuffer) error {
    if buf.HasFormatDescription {
        err := avfw.writeNalu(buf.FormatDescription.PPS)
        if err != nil {
            return err
        }
        err = avfw.writeNalu(buf.FormatDescription.SPS)
        if err != nil {
            return err
        }
    }
    if !buf.HasSampleData() {
        return nil
    }
    return avfw.writeNalus(buf.SampleData)
}

func (avfw ZMQWriter) writeNalus(bytes []byte) error {
    slice := bytes
    for len(slice) > 0 {
        length := binary.BigEndian.Uint32(slice)
        err := avfw.writeNalu(slice[4 : length+4])
        if err != nil {
            return err
        }
        slice = slice[length+4:]
    }
    return nil
}

func (avfw ZMQWriter) writeNalu(naluBytes []byte) error {
    _, err := avfw.bufferWriter.Write(startCode)
    if err != nil {
        return err
    }
    _, err = avfw.bufferWriter.Write(naluBytes)
    if err != nil {
        return err
    }
    return nil
}

func (avfw ZMQWriter) consumeAudio(buffer CMSampleBuffer) error {
    return nil
}
