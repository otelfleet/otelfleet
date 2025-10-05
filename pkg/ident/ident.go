package ident

import (
	"encoding/hex"
	"hash"
	"maps"
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
	rawMac   []string
	name     string
	metadata map[string]string

	hasher hash.Hash
}

var _ Identity = (*macID)(nil)

func (m *macID) uuid() string {
	m.hasher.Write([]byte(m.name))
	m.hasher.Write([]byte(strings.Join(m.rawMac, "")))
	// could extend this to treat some metadata as unique
	return hex.EncodeToString(m.hasher.Sum([]byte{}))
}

func (m *macID) UniqueIdentifier() ID {
	return ID{
		UUID:     m.uuid(),
		Metatada: m.metadata,
	}
}

func IdFromMac(
	hasher hash.Hash,
	name string,
	extraMetadata map[string]string,
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

	maps.Insert(
		extraMetadata,
		maps.All(map[string]string{
			MetadataIDType: IDTypeMac,
		}),
	)
	return &macID{
		rawMac:   macs,
		name:     name,
		metadata: extraMetadata,
		hasher:   hasher,
	}, nil
}
