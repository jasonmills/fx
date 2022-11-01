// Copyright (c) 2022 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package fx

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
)

// Signal represents a operating system process signal.
type Signal struct {
	OS os.Signal
}

// String will render a Signal type as a string suitable for printing.
func (sig Signal) String() string {
	return fmt.Sprintf("%v", sig.OS)
}

type signalReceivers struct {
	last     *Signal
	lastLock sync.RWMutex
	dones    []chan os.Signal
	doneLock sync.RWMutex
}

func (recv *signalReceivers) done() chan os.Signal {
	recv.doneLock.Lock()
	defer recv.doneLock.Unlock()
	recv.lastLock.RLock()
	defer recv.lastLock.RUnlock()

	ch := make(chan os.Signal, 1)

	// if we had received a signal prior to the call of done, send it's
	// os.Signal to the new channel.
	if recv.last != nil {
		ch <- recv.last.OS
	}

	signal.Notify(ch, os.Interrupt, _sigINT, _sigTERM)
	recv.dones = append(recv.dones, ch)
	return ch
}

func (recv *signalReceivers) broadcastDone(signal Signal) (receivers, unsent int) {
	recv.doneLock.RLock()
	defer recv.doneLock.RUnlock()

	receivers = len(recv.dones)

	for _, reader := range recv.dones {
		select {
		case reader <- signal.OS:
		default:
			unsent++
		}
	}

	return
}

type unsentSignalError struct {
	Signal   Signal
	Unsent   int
	Channels int
}

func (err unsentSignalError) Error() string {
	return fmt.Sprintf(
		"send %v signal: %v/%v channels are blocked",
		err.Signal,
		err.Unsent,
		err.Channels,
	)
}

func (recv *signalReceivers) broadcast(signal Signal) (err error) {
	recv.lastLock.Lock()
	recv.last = &signal
	recv.lastLock.Unlock()

	channels, unsent := recv.broadcastDone(signal)

	if unsent != 0 {
		err = unsentSignalError{
			Signal:   signal,
			Channels: channels,
			Unsent:   unsent,
		}
	}

	return
}
