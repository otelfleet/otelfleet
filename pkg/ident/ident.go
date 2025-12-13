package ident

import (
	"encoding/hex"
	"fmt"
	"hash"
	"log/slog"
	"net"
	"sort"
	"strings"
)

const (
	MetadataIDType = "otelfleet.io/id-type"
)

const (
	IDTypeMac = "mac"
)

type ID struct {
	UUID     string
	Metatada map[string]string
}

type Identity interface {
	UniqueIdentifier() ID
}

type macID struct {
	rawMac []string
	name   string

	hasher hash.Hash
}

var _ Identity = (*macID)(nil)

func (m *macID) uuid() string {
	m.hasher.Reset()
	m.hasher.Write([]byte(m.name))
	m.hasher.Write([]byte(strings.Join(m.rawMac, "")))
	// could extend this to treat some metadata as unique
	return hex.EncodeToString(m.hasher.Sum([]byte{}))
}

func (m *macID) UniqueIdentifier() ID {
	return ID{
		UUID: m.uuid(),
	}
}

func IdFromMac(
	hasher hash.Hash,
	name string,
) (Identity, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	// Collect MAC addresses and sort them for consistency
	var macs []string
	for _, intf := range interfaces {
		macs = append(macs, intf.HardwareAddr.String())
	}
	sort.Strings(macs)
	slog.With("macs", len(macs)).Debug(fmt.Sprintf("got mac addresses : %s", strings.Join(macs, ",")))

	return &macID{
		rawMac: macs,
		name:   name,
		hasher: hasher,
	}, nil
}
