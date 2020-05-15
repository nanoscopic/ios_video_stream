TARGET = ios_video_stream

all: $(TARGET)

$(TARGET): main.go server.go go.sum screencapture/coremedia/avfilewriter.go
	go build  -o $(TARGET) .

go.sum:
	go get
	go get .

clean:
	$(RM) $(TARGET) go.sum