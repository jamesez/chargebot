package main

import (
	"encoding/json"
	"flag"
	"log"
	"math"
	"os"
	"strings"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type Message struct {
	Topic   string
	Payload []byte
}

type TasmotaStatus struct {
	Energy struct {
		Power float64 `json:"Power"`
	} `json:"ENERGY"`
}

func main() {
	broker := flag.String("broker", "tcp://192.168.1.5:1883", "Your MQTT btoker")
	user := flag.String("user", "", "MQTT user (optional)")
	password := flag.String("password", "", "MQTT password (optional)")

	monitorTopic := flag.String("mt", "", "Tasmota status topic")
	controlTopic := flag.String("ct", "", "Tasmota control topic")

	flag.Parse()

	if *monitorTopic == "" || *controlTopic == "" {
		log.Println("Invalid setting for -mt or -ct, must not be empty")
		return
	}

	opts := MQTT.NewClientOptions()
	opts.AddBroker(*broker)
	opts.SetClientID("chargebot")
	opts.SetUsername(*user)
	opts.SetPassword(*password)
	opts.SetCleanSession(true)

	stream := make(chan Message)
	stop := make(chan bool, 1)

	// set up callback for messages
	opts.SetDefaultPublishHandler(func(client MQTT.Client, msg MQTT.Message) {
		stream <- Message{
			Topic:   msg.Topic(),
			Payload: msg.Payload(),
		}
	})

	client := MQTT.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	if token := client.Subscribe(*monitorTopic, 0, nil); token.Wait() && token.Error() != nil {
		log.Println(token.Error())
		os.Exit(1)
	}

	// turn on goroutine
	go func() {
		for {
			log.Printf("turning on")
			tasmotaControl(client, controlTopic, true)

			// force off in one hour
			go func() {
				time.Sleep(1 * time.Hour)
				log.Printf("one hour forced turn off")
				tasmotaControl(client, controlTopic, false)
			}()

			// wait six days
			time.Sleep(6 * time.Hour * 24)
		}
	}()

	// montior for charging done
	go func() {
		lastPower := 0.0
		isCharging := false
		lowCount := 0

		for { // ever
			message := <-stream

			// todo: for some reason, we get both STATUS and SENSOR even if we ask for SENSOR?
			if strings.HasSuffix(message.Topic, "SENSOR") {
				var sensor TasmotaStatus
				json.Unmarshal(message.Payload, &sensor)
				log.Printf("message: %+v\n", sensor)

				currentPower := sensor.Energy.Power
				delta := currentPower - lastPower
				deltaPct := math.Abs(delta / lastPower)

				log.Printf("last: %f, current: %f, delta: %f, pct: %0.2f\n", lastPower, currentPower, delta, deltaPct)

				// just high water mark
				if currentPower > lastPower && deltaPct > 0.25 {
					log.Printf("is on\n")
					isCharging = true
					lastPower = (currentPower + lastPower) / 2.0 // average to lag slightly

					// reset lowCount if the port wobbles back high
					if lowCount > 0 {
						log.Printf("resetting lowCount\n")
						lowCount = 0
					}
				}

				// more than 45% change down
				if isCharging && delta < 0 && deltaPct > 0.45 {
					log.Printf("looks like done?\n")

					// wait until it's been low for a bit
					if lowCount <= 10 {
						lowCount++
						log.Printf("waiting: %d\n", lowCount)
						continue
					}

					log.Printf("shutting off\n")
					// turn the port off, reset state
					tasmotaControl(client, controlTopic, false)

					lowCount = 0
					lastPower = 0
					isCharging = false
				}

			}
		}

	}()

	// wait for stop
	<-stop
	client.Disconnect(250)
}

func tasmotaControl(client MQTT.Client, topic *string, power bool) {
	var cmd string
	if power {
		cmd = "ON"
	} else {
		cmd = "OFF"
	}

	token := client.Publish(*topic, 0, false, cmd)
	token.Wait()
}
