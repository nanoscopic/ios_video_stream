package packet

import (
	"encoding/binary"
	"fmt"
	"github.com/danielpaulus/quicktime_video_hack/usb/coremedia"
)

type AsynFeedPacket struct {
	AsyncMagic  uint32
	ClockRef    CFTypeID
	MessageType uint32
	CMSampleBuf coremedia.CMSampleBuffer
}

func NewAsynFeedPacketFromBytes(data []byte) (AsynFeedPacket, error) {
	var packet = AsynFeedPacket{}
	packet.AsyncMagic = binary.LittleEndian.Uint32(data)
	if packet.AsyncMagic != AsynPacketMagic {
		return packet, fmt.Errorf("invalid asyn magic: %x", data)
	}
	packet.ClockRef = binary.LittleEndian.Uint64(data[4:])
	packet.MessageType = binary.LittleEndian.Uint32(data[12:])
	if packet.MessageType != FEED {
		return packet, fmt.Errorf("invalid packet type in asyn feed:%x", data)
	}
	entry, err := coremedia.NewCMSampleBufferFromBytes(data[16:])
	if err != nil {
		return packet, err
	}
	packet.CMSampleBuf = entry
	return packet, nil
}
