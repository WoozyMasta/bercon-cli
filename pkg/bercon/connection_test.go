package bercon

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

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

	_, err = bercon.Send("loadBans")
	if err != nil {
		t.Fatalf("Err %e", err)
	}
	time.Sleep(50 * time.Second)

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
