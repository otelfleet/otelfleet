package deployment

import (
	"context"
	"path"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/deployment/v1alpha1"
	"github.com/otelfleet/otelfleet/pkg/storage/object"
	"github.com/otelfleet/otelfleet/pkg/util/grpcutil"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type Manager interface {
	Get(ctx context.Context, deployID string) (Instance, error)
	Instance(deployID string) Instance

	Exists(ctx context.Context, deployID string) (bool, error)
	Register(ctx context.Context, deployID string, friendlyID string) error

	List(ctx context.Context) ([]Instance, error)
	Delete(ctx context.Context, deployID string) error
}

// FIXME: this is kinda wonky, because we have two competing sources of truth / storage definitions:
// 1. From the connected collector's reporting
// 2. From the control plane specific definitions. E.g. attributes
//
// I should take a serious look later at how we want to actually organize this information.
type Instance interface {
	// control plane
	GetDescription(ctx context.Context) (*v1alpha1.CollectorDescription, error)
	Status(ctx context.Context) (*v1alpha1.CollectorStatus, error)
	History(ctx context.Context, offset, limit uint64) ([]*v1alpha1.EffectiveConfig, error)
	GetConnectionState(ctx context.Context) (*v1alpha1.ConnectionStatus, error)
	GetRemoteStatus(ctx context.Context) (*protobufs.RemoteConfigStatus, error)

	// collector
	SetDescription(ctx context.Context, desc *protobufs.AgentDescription) error
	SetCapabilities(ctx context.Context, capabilities uint64) error
	SetConnectionState(ctx context.Context, state *v1alpha1.ConnectionStatus) error
	SetHealth(ctx context.Context, health *protobufs.ComponentHealth) error
	SetEffectiveConfig(ctx context.Context, config *protobufs.EffectiveConfig) error
	SetRemoteStatus(ctx context.Context, config *protobufs.RemoteConfigStatus) error
}

type manager struct {
	genericStorage object.TypeURLStore
}

func NewManager(genericStorage object.TypeURLStore) Manager {
	return &manager{genericStorage: genericStorage}
}

var (
	_ Manager  = (*manager)(nil)
	_ Instance = (*instance)(nil)
)

func (m *manager) Instance(deployID string) Instance {
	return &instance{
		deployID:       deployID,
		genericStorage: m.genericStorage,
	}
}

func (m *manager) Get(ctx context.Context, deployID string) (Instance, error) {
	if _, err := getProto[*v1alpha1.CollectorDescription](ctx, m.genericStorage, deployID); err != nil {
		return nil, err
	}
	return m.Instance(deployID), nil
}

func (m *manager) Exists(ctx context.Context, deployID string) (bool, error) {
	_, err := getProto[*v1alpha1.CollectorDescription](ctx, m.genericStorage, deployID)
	if grpcutil.IsErrorNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (m *manager) Register(ctx context.Context, deployID string, friendlyID string) error {
	return putProto(ctx, m.genericStorage, deployID, &v1alpha1.CollectorDescription{
		Id:           deployID,
		FriendlyName: friendlyID,
	})
}

func (m *manager) List(ctx context.Context) ([]Instance, error) {
	keys, err := m.genericStorage.ListKeys(ctx, typeURL[*v1alpha1.CollectorDescription]())
	if err != nil {
		return nil, err
	}
	instances := make([]Instance, 0, len(keys))
	for _, key := range keys {
		instances = append(instances, m.Instance(path.Base(key)))
	}
	return instances, nil
}

func (m *manager) Delete(ctx context.Context, deployID string) error {
	typeURLs := []string{
		typeURL[*protobufs.AgentDescription](),
		typeURL[*v1alpha1.ConnectionStatus](),
		typeURL[*protobufs.ComponentHealth](),
		typeURL[*protobufs.EffectiveConfig](),
		typeURL[*protobufs.RemoteConfigStatus](),
		typeURL[*v1alpha1.CollectorDescription](),
	}
	for _, url := range typeURLs {
		if err := m.genericStorage.Delete(ctx, url, deployID); err != nil {
			return err
		}
	}
	return nil
}

type instance struct {
	deployID       string
	genericStorage object.TypeURLStore
}

func (i *instance) capabiliesKey(deployID string) string {
	return deployID + "-capabilies"
}

func (i *instance) GetDescription(ctx context.Context) (*v1alpha1.CollectorDescription, error) {
	// FIXME: maybe we want to return partial messages on error?

	desc, err := getProto[*v1alpha1.CollectorDescription](ctx, i.genericStorage, i.deployID)
	if err != nil {
		return nil, err
	}
	attrs, err := getProtoOrNil[*protobufs.AgentDescription](ctx, i.genericStorage, i.deployID)
	if err != nil {
		return nil, err
	}
	desc.IdentifyingAttributes = convertKeyValues(attrs.GetIdentifyingAttributes())
	desc.NonIdentifyingAttributes = convertKeyValues(attrs.GetNonIdentifyingAttributes())

	cap, err := getProto[*wrapperspb.UInt64Value](ctx, i.genericStorage, i.capabiliesKey(i.deployID))
	if grpcutil.IsErrorNotFound(err) {
		cap = wrapperspb.UInt64(uint64(0))
	} else if err != nil {
		return nil, err
	}
	desc.Capabilities = Capabilities(cap.GetValue()).ToStringSlice()
	return desc, nil
}

func (i *instance) Status(ctx context.Context) (*v1alpha1.CollectorStatus, error) {
	health, err := getProtoOrNil[*protobufs.ComponentHealth](ctx, i.genericStorage, i.deployID)
	if err != nil {
		return nil, err
	}
	effective, err := getProtoOrNil[*protobufs.EffectiveConfig](ctx, i.genericStorage, i.deployID)
	if err != nil {
		return nil, err
	}
	remote, err := getProtoOrNil[*protobufs.RemoteConfigStatus](ctx, i.genericStorage, i.deployID)
	if err != nil {
		return nil, err
	}
	conn, err := getProtoOrNil[*v1alpha1.ConnectionStatus](ctx, i.genericStorage, i.deployID)
	if err != nil {
		return nil, err
	}
	return &v1alpha1.CollectorStatus{
		Health:             convertHealth(health),
		EffectiveConfig:    convertEffectiveConfig(effective),
		RemoteConfigStatus: convertRemoteStatus(remote),
		ConnStatus:         conn,
	}, nil
}

func (i *instance) History(ctx context.Context, offset, limit uint64) ([]*v1alpha1.EffectiveConfig, error) {
	configs, err := historyProto[*protobufs.EffectiveConfig](ctx, i.genericStorage, i.deployID, offset, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*v1alpha1.EffectiveConfig, 0, len(configs))
	for _, config := range configs {
		out = append(out, convertEffectiveConfig(config))
	}
	return out, nil
}

func (i *instance) GetConnectionState(ctx context.Context) (*v1alpha1.ConnectionStatus, error) {
	return getProto[*v1alpha1.ConnectionStatus](ctx, i.genericStorage, i.deployID)
}

func (i *instance) GetRemoteStatus(ctx context.Context) (*protobufs.RemoteConfigStatus, error) {
	return getProto[*protobufs.RemoteConfigStatus](ctx, i.genericStorage, i.deployID)
}

func (i *instance) SetDescription(ctx context.Context, desc *protobufs.AgentDescription) error {
	return putProto(ctx, i.genericStorage, i.deployID, desc)
}

func (i *instance) SetCapabilities(ctx context.Context, cap uint64) error {
	return putProto(ctx, i.genericStorage, i.capabiliesKey(i.deployID), wrapperspb.UInt64(cap))
}

func (i *instance) SetConnectionState(ctx context.Context, state *v1alpha1.ConnectionStatus) error {
	return putProto(ctx, i.genericStorage, i.deployID, state)
}

func (i *instance) SetHealth(ctx context.Context, health *protobufs.ComponentHealth) error {
	return putProto(ctx, i.genericStorage, i.deployID, health)
}

func (i *instance) SetEffectiveConfig(ctx context.Context, config *protobufs.EffectiveConfig) error {
	return putProto(ctx, i.genericStorage, i.deployID, config)
}

func (i *instance) SetRemoteStatus(ctx context.Context, config *protobufs.RemoteConfigStatus) error {
	return putProto(ctx, i.genericStorage, i.deployID, config)
}
