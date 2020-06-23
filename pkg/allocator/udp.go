package allocator

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	backoff "github.com/cenkalti/backoff/v4"

	"k8s.io/klog"
)

// RunUDPLoad runs many concurrent game connections on a simple UDP server
// This is designed to test the allocator service and autoscaling of the game servers.
func (c *Client) RunUDPLoad(count int, delay int, duration int) error {
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go c.testUDP(i, &wg, duration)
		time.Sleep(time.Duration(delay) * time.Second)
	}
	wg.Wait()
	return nil
}

func (c *Client) testUDP(id int, wg *sync.WaitGroup, duration int) {
	defer wg.Done()
	maxRetries := 10

	var a *Allocation
	var err error

	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Duration(1 * time.Second)

	i := 0
	for {
		if i == maxRetries {
			klog.Errorf("max retries (%d) reached", maxRetries)
			return
		}
		i++
		delay := b.NextBackOff()
		a, err = c.AllocateGameserver()
		if err != nil {
			klog.Infof("%d %s - retrying in %fs", id, err.Error(), delay.Seconds())
			time.Sleep(delay)
			continue
		} else {
			break
		}
	}

	klog.Infof("%d - got allocation %s %d. Proceeding to connection...\n", id, a.Address, a.Port)
	err = a.testUDP(id, duration)
	if err != nil {
		klog.Error(err)
	}
}

// testUDP tests a series of connections to the simple-udp server gameserver example
func (a *Allocation) testUDP(id int, duration int) error {
	endpoint := fmt.Sprintf("%s:%d", a.Address, a.Port)

	conn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	dst, err := net.ResolveUDPAddr("udp", endpoint)
	if err != nil {
		log.Fatal(err)
	}

	klog.Infof("%d - connected to gameserver and sending hello", id)

	msg := fmt.Sprintf("Hello from process %d!", id)
	_, err = conn.WriteTo([]byte(msg), dst)
	if err != nil {
		return err
	}
	klog.Infof("%d - sleeping %d seconds to view logs", id, duration)
	time.Sleep(time.Duration(duration) * time.Second)
	klog.Infof("%d - sending EXIT command", id)

	_, err = conn.WriteTo([]byte("EXIT"), dst)
	if err != nil {
		return err
	}
	return nil
}
