package sensu

import (
	"github.com/streadway/amqp"
	"log"
	"time"
)

type Processor interface {
	Start()
	Stop()
	Restart()
	Close()
}

type Client struct {
	config    *Config
	q         MessageQueuer
	processes []Processor
}

func NewClient(c *Config) *Client {
	return &Client{
		config:  c,
	}
}

func (c *Client) Start(errc chan error) {
	var disconnected chan *amqp.Error
	connected := make(chan bool)
	init := true

	c.q = NewRabbitmq(c.config.Rabbitmq)
	go c.q.Connect(connected, errc)

	c.processes = []Processor{NewKeepalive(c.q, 5 * time.Second)}


	for {
		select {
		case <-connected:
			for _, proc := range c.processes {
				if init {
					go proc.Start()
					init = false
				} else {
					go proc.Restart()
				}
			}
			// Enable disconnect channel
			disconnected = c.q.Disconnected()
		case errd := <-disconnected:
			// Disable disconnect channel
			disconnected = nil

			log.Printf("RabbitMQ disconnected: %s", errd)
			c.Reset()

			time.Sleep(10 * time.Second)
			go c.q.Connect(connected, errc)
		}
	}
}

func (c *Client) Reset() chan error {
	for _, proc := range c.processes {
		proc.Stop()
	}
	return nil
}

func (c *Client) Shutdown() chan error {
	// Stop keepalive timer
	for _, proc := range c.processes {
		proc.Close()
	}
	return nil
}
