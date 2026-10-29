//go:build windows

package platform

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// The local channel opens only for the allowed user, and the service can tell
// which process is at the other end (the helper it launched).
func TestLocalChannelAndPeerPID(t *testing.T) {
	tok := windows.GetCurrentProcessToken()
	u, err := tok.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("aula-prueba-%d-%d", os.Getpid(), time.Now().UnixNano())
	l, err := ListenLocal(name, u.User.Sid.String())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	accepted := make(chan int, 1)
	go func() {
		c, err := l.Accept()
		if err != nil {
			accepted <- -1
			return
		}
		defer c.Close()
		pid, err := PeerPID(c)
		if err != nil {
			pid = -2
		}
		line, _ := bufio.NewReader(c).ReadString('\n')
		if line != "hola\n" {
			pid = -3
		}
		accepted <- pid
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := DialLocal(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte("hola\n")); err != nil {
		t.Fatal(err)
	}
	if pid := <-accepted; pid != os.Getpid() {
		t.Fatalf("peer pid = %d, want %d", pid, os.Getpid())
	}
	if _, err := ListenLocal(name+"-x", "everyone"); err == nil {
		t.Fatal("accepted a channel without a SID")
	}
}

// Status works for a standard user, without installing anything.
func TestServiceStatusReadOnly(t *testing.T) {
	st, err := NewServiceControl("AulaNoExiste-" + fmt.Sprint(os.Getpid())).Status()
	if err != nil || st.State != ServiceNotInstalled {
		t.Fatalf("missing service: %+v, %v", st, err)
	}
	// The DNS client runs on every Windows, automatically.
	st, err = NewServiceControl("Dnscache").Status()
	if err != nil {
		t.Skipf("Dnscache: %v", err)
	}
	if st.State != ServiceRunning || st.PID == 0 || !st.Auto {
		t.Fatalf("Dnscache status %+v", st)
	}
	if is, err := IsService(); err != nil || is {
		t.Fatalf("IsService in a test = %v, %v", is, err)
	}
}
