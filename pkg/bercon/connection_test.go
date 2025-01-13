package bercon

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func initVars() (address, password string) {
	ip, ok := os.LookupEnv("BERCON_ADDRESS")
	if !ok {
		ip = "127.0.0.1"
	}
	port, ok := os.LookupEnv("BERCON_PORT")
	if !ok {
		port = "2025"
	}
	address = fmt.Sprintf("%s:%s", ip, port)

	password, ok = os.LookupEnv("BERCON_PASSWORD")
	if !ok {
		password = ""
	}

	return
}

func TestBercon(t *testing.T) {
	address, password := initVars()
	bercon, err := Open(address, password)
	if err != nil {
		t.Fatalf("Err %e", err)
	}

	players, err := bercon.Send("players")
	if err != nil {
		t.Fatalf("Err %e", err)
	}
	fmt.Println(string(players))

	bercon.Close()
}

func TestBerconAlive(t *testing.T) {
	address, password := initVars()
	bercon, err := Open(address, password)
	if err != nil {
		t.Fatalf("Err %e", err)
	}
	bercon.SetKeepaliveTimeout(10)
	bercon.StartKeepAlive()

	go func() {
		for msg := range bercon.Messages {
			fmt.Printf(
				"Got packet seq=%d data=\"%s\" time=\"%s\"\n",
				msg.Seq, string(msg.Data), msg.Time.Format(time.Stamp),
			)
		}
	}()

	time.Sleep(10 * time.Second)

	_, err = bercon.Send("loadBans")
	if err != nil {
		t.Fatalf("Err %e", err)
	}

	bercon2, err := Open(address, password)
	if err != nil {
		t.Fatalf("Err %e", err)
	}
	_, err = bercon2.Send("commands")
	if err != nil {
		t.Fatalf("Err %e", err)
	}
	bercon2.Close()

	time.Sleep(20 * time.Second)

	bercon3, err := Open(address, password)
	if err != nil {
		t.Fatalf("Err %e", err)
	}
	bercon3.Close()

	time.Sleep(20 * time.Second)

	data, err := bercon.Send("bans")
	if err != nil {
		t.Fatalf("Err %e", err)
	}
	fmt.Println(string(data))

	bercon.Close()
}

func TestBerconLoop(t *testing.T) {
	address, password := initVars()
	bercon, err := Open(address, password)
	if err != nil {
		t.Fatalf("Err %e", err)
	}
	bercon.SetKeepaliveTimeout(1)
	bercon.StartKeepAlive()

	cnt := 10
	commands := []string{"players", "bans", "admins"}

	var wg sync.WaitGroup
	errorCh := make(chan error, len(commands))

	executeCommand := func(command string, threadID int) {
		defer wg.Done()

		for i := 0; i < cnt; i++ {
			fmt.Printf("Thread %d - Execute %d/%d: %s\n", threadID, i+1, cnt, command)

			_, err := bercon.Send(command)
			if err != nil {
				errorCh <- fmt.Errorf("Thread %d - Error executing command '%s': %v", threadID, command, err)
				return
			}

			time.Sleep(time.Millisecond * 500)
		}
	}

	for i, command := range commands {
		wg.Add(1)
		go executeCommand(command, i+1)
	}

	go func() {
		wg.Wait()
		close(errorCh)
	}()

	for err := range errorCh {
		if err != nil {
			t.Error(err)
		}
	}

	bercon.Close()
}
