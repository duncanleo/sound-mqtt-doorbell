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
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	playedFiles = []string{}
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

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

// pickSoundFile pick a sound file from a path that is possibly either a file or folder.
// if it is a folder, it will randomly pick a file within
func pickSoundFile(soundPath string) (string, error) {
	if len(soundPath) == 0 {
		return "", fmt.Errorf("File/folder path is empty")
	}
	var chosenPath = ""
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

		// Filter out irrelevant files
		var relevantFiles = []os.FileInfo{}
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), ".") {
				relevantFiles = append(relevantFiles, file)
			}
		}

		if len(playedFiles) >= len(relevantFiles) {
			playedFiles = []string{}
		}

		for {
			chosenPath = path.Join(soundPath, relevantFiles[rand.Intn(len(relevantFiles))].Name())

			if !contains(playedFiles, chosenPath) {
				break
			}
		}

		playedFiles = append(playedFiles, chosenPath)
	} else {
		return chosenPath, fmt.Errorf("Path '%s' is not a directory", soundPath)
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

		eventChan <- string(msg.Payload())
	})

	for event := range eventChan {
		if isPlaying || event != "ON" {
			time.Sleep(1 * time.Second)
			continue
		}

		soundFile, err := pickSoundFile(*soundPath)
		if err != nil {
			log.Println(err)
			time.Sleep(1 * time.Second)
			continue
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
			"-af",
			"dynaudnorm=f=100:g=15:n=1:p=1",
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
