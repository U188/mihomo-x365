package vless

import (
	"net"
	"strings"

	"github.com/metacubex/mihomo/common/utils"

	"github.com/gofrs/uuid/v5"
)

const (
	XRO = "xtls-rprx-origin"
	XRD = "xtls-rprx-direct"
	XRS = "xtls-rprx-splice"
	XRV = "xtls-rprx-vision"

	Version byte = 0 // protocol version. preview version is 0
)

// Command types
const (
	CommandTCP byte = 1
	CommandUDP byte = 2
	CommandMux byte = 3
)

// Addr types
const (
	AtypIPv4       byte = 1
	AtypDomainName byte = 2
	AtypIPv6       byte = 3
)

// DstAddr store destination address
type DstAddr struct {
	UDP      bool
	AddrType byte
	Addr     []byte
	Port     uint16
	Mux      bool // currently used for XUDP only
}

// X365Marker is the uuid suffix that switches the connection to the private
// "X365" wire format. It is not part of the uuid itself.
const X365Marker = "#x365"

// Client is vless connection generator
type Client struct {
	uuid   uuid.UUID
	Addons *Addons
	IsX365 bool // use the private X365 wire format
}

// StreamConn return a Conn with net.Conn and DstAddr
func (c *Client) StreamConn(conn net.Conn, dst *DstAddr) (net.Conn, error) {
	return newConn(conn, c, dst)
}

func (c *Client) PacketConn(conn net.Conn, rAddr net.Addr) net.PacketConn {
	return &PacketConn{conn, rAddr}
}

// parseX365UUID strips the "#x365" marker from the uuid and reports whether it
// was present. Without stripping, UUIDMap would fall back to a derived UUIDv5
// of the whole string and authentication against the server would fail.
func parseX365UUID(uuidStr string) (string, bool) {
	if strings.HasSuffix(strings.ToLower(uuidStr), X365Marker) {
		return uuidStr[:len(uuidStr)-len(X365Marker)], true
	}
	return uuidStr, false
}

// IsX365UUID reports whether the uuid carries the private "#x365" marker so
// adapter-level behavior (e.g. the endpoint-specific User-Agent) can match
// the wire-format switch inside NewClient.
func IsX365UUID(uuidStr string) bool {
	_, ok := parseX365UUID(uuidStr)
	return ok
}

// NewClient return Client instance
func NewClient(uuidStr string, addons *Addons) (*Client, error) {
	cleanID, isX365 := parseX365UUID(uuidStr)
	uid := utils.UUIDMap(cleanID)

	return &Client{
		uuid:   uid,
		Addons: addons,
		IsX365: isX365,
	}, nil
}
