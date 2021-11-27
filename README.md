# sound-mqtt-doorbell
This is a simple command line utility built in Go intended for doorbell use, and primarily for deployment to a Raspberry Pi.

It connects to an MQTT server and subscribes to a single topic. When "ON" is written to the topic, a song will be randomly chosen from a folder and played.

Note that this requires these Linux dependencies:
- ffmpeg
- aplay

```
Usage of sound-mqtt-doorbell:
  -brokerURI string
    	URI of the MQTT broker (default "mqtt://127.0.0.1:1883")
  -clientID string
    	client ID for MQTT (default "sound-mqtt-doorbell")
  -soundPath string
    	path to a file/folder that is a/contain sound files. If it is a folder path, it will randomise and pick one file.
  -topic string
    	MQTT topic to subscribe to (default "sound-mqtt-doorbell")
```
