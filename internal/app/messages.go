// SPDX-License-Identifier: MIT
// Copyright (c) 2024-2026 WoozyMasta
// Source: github.com/woozymasta/bercon-cli

package app

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/woozymasta/bercon"
	"github.com/woozymasta/bercon-cli/internal/output"
)

// responseMatcher tracks one command whose useful payload may arrive asynchronously.
type responseMatcher struct {
	matched chan bercon.Message // matched carries at most one candidate response for the active command.
	command string              // command selects the parser schema expected from asynchronous payloads.
	held    bercon.Message      // held keeps the candidate out of the event stream until response arbitration finishes.
	hasHeld bool                // hasHeld distinguishes an empty payload from the absence of a candidate.
}

// rconClient is the transport surface used by command and message routing.
type rconClient interface {
	Command(context.Context, string) ([]byte, error) // Command sends one command and returns its direct response.
	Messages() <-chan bercon.Message                 // Messages exposes asynchronous server messages.
	DroppedMessages() uint64                         // DroppedMessages reports transport messages lost before CLI routing.
	Done() <-chan struct{}                           // Done closes when the client session ends.
	Err() error                                      // Err reports why the client session ended.
}

// messageBroker is the only goroutine that reads Client.Messages.
type messageBroker struct {
	client  rconClient          // client is the sole source of server messages for this process mode.
	events  chan bercon.Message // events preserves unmatched messages for the selected output mode.
	stop    chan struct{}       // stop requests the reader goroutine to exit.
	stopped chan struct{}       // stopped closes after run has released its resources.
	active  *responseMatcher    // active is the only command eligible to consume a recognized response.
	dropped atomic.Uint64       // dropped counts events lost from the bounded application stream.
	once    sync.Once           // once makes shutdown safe when cleanup paths overlap.
	mu      sync.Mutex          // mu protects active while a command matcher is installed or removed.
	forward bool                // forward controls whether unmatched events reach the mode output stream.
}

// synchronizedWriter prevents event output from racing with command output.
type synchronizedWriter struct {
	w  io.Writer  // w is the caller-selected output stream.
	mu sync.Mutex // mu prevents output from interleaving across command and event goroutines.
}

// Write serializes complete writes from the command and event paths.
func (w *synchronizedWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.w.Write(data)
}

// newMessageBroker creates a bounded event stream and one matcher slot.
func newMessageBroker(client rconClient, forwardEvents bool) *messageBroker {
	return &messageBroker{
		client: client, events: make(chan bercon.Message, 128), forward: forwardEvents,
		stop: make(chan struct{}), stopped: make(chan struct{}),
	}
}

// run drains transport events and never blocks the client queue.
func (b *messageBroker) run() {
	defer close(b.stopped)
	for {
		select {
		case <-b.stop:
			return

		case message, ok := <-b.client.Messages():
			if !ok {
				return
			}

			message.Data = append([]byte(nil), message.Data...)
			b.publish(message)
		}
	}
}

// publish forwards a copy and offers recognized responses to the current waiter.
func (b *messageBroker) publish(message bercon.Message) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.active != nil && !b.active.hasHeld && output.Parse(message.Data, b.active.command).Known {
		b.active.held = message
		b.active.hasHeld = true
		select {
		case b.active.matched <- message:
		default:
		}
		return
	}

	b.publishEventLocked(message)
}

// publishEventLocked keeps event delivery non-blocking while the broker mutex is held.
func (b *messageBroker) publishEventLocked(message bercon.Message) {
	if !b.forward {
		return
	}

	select {
	case b.events <- message:
	default:
		b.dropped.Add(1)
	}
}

// match registers before a command is sent to avoid losing fast async responses.
func (b *messageBroker) match(command string) <-chan bercon.Message {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(command)))
	if len(fields) == 0 {
		return nil
	}

	switch fields[0] {
	case "players", "#players", "admins", "bans":
	default:
		return nil
	}

	matcher := &responseMatcher{command: command, matched: make(chan bercon.Message, 1)}
	b.mu.Lock()
	b.active = matcher
	b.mu.Unlock()

	return matcher.matched
}

// clear removes only the matcher associated with the completed request.
func (b *messageBroker) clear(waiter <-chan bercon.Message, forwardMatched bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.active != nil && (<-chan bercon.Message)(b.active.matched) == waiter {
		if forwardMatched && b.active.hasHeld {
			b.publishEventLocked(b.active.held)
		}
		b.active = nil
	}
}

// close stops the reader and waits for it to exit.
func (b *messageBroker) close() {
	b.once.Do(func() {
		close(b.stop)
		<-b.stopped

		b.mu.Lock()
		close(b.events)
		b.mu.Unlock()
	})
}

// reportDroppedMessages reports queue loss without consuming the event stream.
func reportDroppedMessages(ctx context.Context, broker *messageBroker, stderr io.Writer) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var reported uint64

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			dropped := broker.client.DroppedMessages() + broker.dropped.Load()
			if dropped > reported {
				_, _ = fmt.Fprintf(stderr, "warning: dropped %d asynchronous server messages\n", dropped-reported)
				reported = dropped
			}
		}
	}
}

// streamMessages writes asynchronous events and optionally prefixes their local receive time.
func streamMessages(ctx context.Context, events <-chan bercon.Message, stdout io.Writer, timestamps bool) {
	for {
		select {
		case <-ctx.Done():
			return

		case message, ok := <-events:
			if !ok {
				return
			}

			payload := append([]byte(nil), message.Data...)
			if timestamps {
				prefix := "[" + time.Now().Format(time.RFC3339) + "] "
				payload = append([]byte(prefix), payload...)
			}
			if len(message.Data) == 0 || message.Data[len(message.Data)-1] != '\n' {
				payload = append(payload, '\n')
			}
			_, _ = stdout.Write(payload)
		}
	}
}
