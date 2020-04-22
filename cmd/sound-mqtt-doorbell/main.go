package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func connect(clientID string, uri *url.URL) (mqtt.Client, error) {
	var opts = mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetUsername(uri.User.Username())
	password, _ := uri.User.Password()
	opts.SetPassword(password)
	opts.SetClientID(clientID)
	opts.CleanSession = false

	var client = mqtt.NewClient(opts)
	var token = client.Connect()
	for !token.WaitTimeout(3 * time.Second) {
	}
	return client, token.Error()
}

// pickSoundFile pick a sound file from a path that is possibly either a file or folder.
// if it is a folder, it will randomly pick a file within
func pickSoundFile(soundPath string) (string, error) {
	if len(soundPath) == 0 {
		return "", fmt.Errorf("File/folder path is empty")
	}
	var chosenPath = soundPath
	pathInfo, err := os.Stat(soundPath)
	if err != nil {
		return chosenPath, err
	} else if os.IsNotExist(err) {
		return chosenPath, fmt.Errorf("File/folder path '%s' does not exist", soundPath)
	}
	if pathInfo.IsDir() {
		files, err := ioutil.ReadDir(soundPath)
		if err != nil {
			return chosenPath, err
		}
		chosenPath = path.Join(soundPath, files[rand.Intn(len(files))].Name())
	}
	return chosenPath, nil
}

var isPlaying = false

func main() {
	var brokerURI = flag.String("brokerURI", "mqtt://127.0.0.1:1883", "URI of the MQTT broker")
	var clientID = flag.String("clientID", "sound-mqtt-doorbell", "client ID for MQTT")
	var topic = flag.String("topic", "sound-mqtt-doorbell", "MQTT topic to subscribe to")
	var soundPath = flag.String("soundPath", "", "path to a file/folder that is a/contain sound files. If it is a folder path, it will randomise and pick one file.")

	flag.Parse()

	mqttURI, err := url.Parse(*brokerURI)
	if err != nil {
		log.Fatal(err)
	}

	client, err := connect(*clientID, mqttURI)
	if err != nil {
		log.Fatal(err)
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("cleanup")
		client.Disconnect(0)
		os.Exit(1)
	}()

	var eventChan = make(chan string, 0)

	client.Subscribe(*topic, 0, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("[%s]: %s\n", *topic, string(msg.Payload()))

		eventChan <- "press"
	})

	for range eventChan {
		if isPlaying {
			return
		}

		soundFile, err := pickSoundFile(*soundPath)
		if err != nil {
			log.Println(err)
			return
		}
		log.Printf("Playing sound file '%s'\n", soundFile)

		var aplayArgs = []string{
			"-f",
			"cd",
		}

		aplay := exec.Command("aplay", aplayArgs...)
		aplay.Stdout = os.Stdout
		aplay.Stderr = os.Stderr

		var ffmpegArgs = []string{
			"-i",
			soundFile,
			"-f",
			"s16le",
			"-c:a",
			"pcm_s16le",
			"-r",
			"44100",
			"-",
		}
		cmd := exec.Command("ffmpeg", ffmpegArgs...)
		aplay.Stdin, _ = cmd.StdoutPipe()
		cmd.Stderr = os.Stderr
		isPlaying = true

		aplay.Start()
		cmd.Run()
		aplay.Wait()

		isPlaying = false
	}
}
