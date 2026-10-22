package vless

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/metacubex/mihomo/common/utils"
)

const x365TestUUID = "84146b14-4a2c-4772-b24c-6ef6fd21499a"

func TestParseX365UUID(t *testing.T) {
	clean, ok := parseX365UUID(x365TestUUID + "#x365")
	if !ok {
		t.Fatal("#x365 suffix not detected")
	}
	if clean != x365TestUUID {
		t.Fatalf("clean uuid = %q, want %q", clean, x365TestUUID)
	}

	clean, ok = parseX365UUID(x365TestUUID)
	if ok || clean != x365TestUUID {
		t.Fatalf("plain uuid must be untouched, got %q ok=%v", clean, ok)
	}

	clean, ok = parseX365UUID("prefix#x365mid")
	if ok {
		t.Fatal("infix marker must not be detected")
	}
}

func TestNewClientX365(t *testing.T) {
	client, err := NewClient(x365TestUUID+"#x365", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !client.IsX365 {
		t.Fatal("IsX365 must be true for #x365 suffix")
	}
	want := utils.UUIDMap(x365TestUUID)
	if client.uuid != want {
		t.Fatalf("uuid = %v, want %v (marker must not feed UUIDMap)", client.uuid, want)
	}

	client, err = NewClient(x365TestUUID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if client.IsX365 {
		t.Fatal("IsX365 must be false without suffix")
	}
}

func TestX365WireFormat(t *testing.T) {
	client, err := NewClient(x365TestUUID+"#x365", nil)
	if err != nil {
		t.Fatal(err)
	}

	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	payload := []byte("GET / HTTP/1.1\r\nHost: dldir1.qq.com\r\n\r\n")

	serverErr := make(chan error, 1)
	go func() {
		defer b.Close()
		// expected request layout
		var want bytes.Buffer
		want.Write([]byte{'X', '3', '6', '5', 0x01})
		want.WriteByte(CommandTCP)
		uid := utils.UUIDMap(x365TestUUID)
		want.Write(uid.Bytes())
		binary.Write(&want, binary.BigEndian, uint16(443))
		want.WriteByte(AtypDomainName)
		host := []byte("dldir1.qq.com")
		want.WriteByte(byte(len(host)))
		want.Write(host)
		want.Write(payload)

		got := make([]byte, want.Len())
		b.SetReadDeadline(time.Now().Add(5 * time.Second))
		if _, err := io.ReadFull(b, got); err != nil {
			serverErr <- err
			return
		}
		if !bytes.Equal(got, want.Bytes()) {
			serverErr <- fmt.Errorf("x365 request mismatch:\n got %x\nwant %x", got, want.Bytes())
			return
		}

		// handshake response
		b.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := b.Write([]byte{'X', '3', '6', '5', 0x00}); err != nil {
			serverErr <- err
			return
		}
		if _, err := b.Write([]byte("HTTP/1.1 200 OK")); err != nil {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	conn, err := client.StreamConn(a, &DstAddr{
		UDP:      false,
		AddrType: AtypDomainName,
		Addr:     append([]byte{13}, []byte("dldir1.qq.com")...),
		Port:     443,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	got := make([]byte, len("HTTP/1.1 200 OK"))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != "HTTP/1.1 200 OK" {
		t.Fatalf("read back %q", got)
	}

	if err := <-serverErr; err != nil {
		t.Fatal(err)
	}
}
