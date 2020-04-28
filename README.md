[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 1. What is this?
This is an altered version [Quicktime Video Hack](https://github.com/danielpaulus/quicktime_videoHack)

It has been altered in the following ways:

1. All gstreamer code has been removed
2. The h264 stream is not decoded at all. Instead it is send via ZMQ elsewhere for decoding.
3. It receives decoded frames back via ZMQ also in the form of a series of jpegs.
4. Those jpegs are then served out via http ( both latest jpeg, and streaming websocket )

## 2. Installation

1. `make`

## 3. Usage

1. Start a decoder
2. `./ios_video_stream stream [zmq spec to send to] [zmq spec to recv from] [--udid=<udid>]
