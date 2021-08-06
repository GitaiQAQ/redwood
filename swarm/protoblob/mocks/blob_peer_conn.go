// Code generated by mockery v2.8.0. DO NOT EDIT.

package mocks

import (
	context "context"

	blob "redwood.dev/blob"

	crypto "redwood.dev/crypto"

	mock "github.com/stretchr/testify/mock"

	protoblob "redwood.dev/swarm/protoblob"

	swarm "redwood.dev/swarm"

	time "time"

	types "redwood.dev/types"

	utils "redwood.dev/utils"
)

// BlobPeerConn is an autogenerated mock type for the BlobPeerConn type
type BlobPeerConn struct {
	mock.Mock
}

// AddStateURI provides a mock function with given fields: stateURI
func (_m *BlobPeerConn) AddStateURI(stateURI string) {
	_m.Called(stateURI)
}

// Addresses provides a mock function with given fields:
func (_m *BlobPeerConn) Addresses() []types.Address {
	ret := _m.Called()

	var r0 []types.Address
	if rf, ok := ret.Get(0).(func() []types.Address); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.Address)
		}
	}

	return r0
}

// AnnouncePeers provides a mock function with given fields: ctx, peerDialInfos
func (_m *BlobPeerConn) AnnouncePeers(ctx context.Context, peerDialInfos []swarm.PeerDialInfo) error {
	ret := _m.Called(ctx, peerDialInfos)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []swarm.PeerDialInfo) error); ok {
		r0 = rf(ctx, peerDialInfos)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Close provides a mock function with given fields:
func (_m *BlobPeerConn) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DialInfo provides a mock function with given fields:
func (_m *BlobPeerConn) DialInfo() swarm.PeerDialInfo {
	ret := _m.Called()

	var r0 swarm.PeerDialInfo
	if rf, ok := ret.Get(0).(func() swarm.PeerDialInfo); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(swarm.PeerDialInfo)
	}

	return r0
}

// EnsureConnected provides a mock function with given fields: ctx
func (_m *BlobPeerConn) EnsureConnected(ctx context.Context) error {
	ret := _m.Called(ctx)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Failures provides a mock function with given fields:
func (_m *BlobPeerConn) Failures() uint64 {
	ret := _m.Called()

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// FetchBlob provides a mock function with given fields: blobID
func (_m *BlobPeerConn) FetchBlob(blobID blob.ID) error {
	ret := _m.Called(blobID)

	var r0 error
	if rf, ok := ret.Get(0).(func(blob.ID) error); ok {
		r0 = rf(blobID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// LastContact provides a mock function with given fields:
func (_m *BlobPeerConn) LastContact() time.Time {
	ret := _m.Called()

	var r0 time.Time
	if rf, ok := ret.Get(0).(func() time.Time); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(time.Time)
	}

	return r0
}

// LastFailure provides a mock function with given fields:
func (_m *BlobPeerConn) LastFailure() time.Time {
	ret := _m.Called()

	var r0 time.Time
	if rf, ok := ret.Get(0).(func() time.Time); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(time.Time)
	}

	return r0
}

// PublicKeys provides a mock function with given fields: addr
func (_m *BlobPeerConn) PublicKeys(addr types.Address) (crypto.SigningPublicKey, crypto.AsymEncPubkey) {
	ret := _m.Called(addr)

	var r0 crypto.SigningPublicKey
	if rf, ok := ret.Get(0).(func(types.Address) crypto.SigningPublicKey); ok {
		r0 = rf(addr)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(crypto.SigningPublicKey)
		}
	}

	var r1 crypto.AsymEncPubkey
	if rf, ok := ret.Get(1).(func(types.Address) crypto.AsymEncPubkey); ok {
		r1 = rf(addr)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(crypto.AsymEncPubkey)
		}
	}

	return r0, r1
}

// Ready provides a mock function with given fields:
func (_m *BlobPeerConn) Ready() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// ReceiveBlobHeader provides a mock function with given fields:
func (_m *BlobPeerConn) ReceiveBlobHeader() (protoblob.FetchBlobResponseHeader, error) {
	ret := _m.Called()

	var r0 protoblob.FetchBlobResponseHeader
	if rf, ok := ret.Get(0).(func() protoblob.FetchBlobResponseHeader); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(protoblob.FetchBlobResponseHeader)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ReceiveBlobPacket provides a mock function with given fields:
func (_m *BlobPeerConn) ReceiveBlobPacket() (protoblob.FetchBlobResponseBody, error) {
	ret := _m.Called()

	var r0 protoblob.FetchBlobResponseBody
	if rf, ok := ret.Get(0).(func() protoblob.FetchBlobResponseBody); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(protoblob.FetchBlobResponseBody)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RemainingBackoff provides a mock function with given fields:
func (_m *BlobPeerConn) RemainingBackoff() time.Duration {
	ret := _m.Called()

	var r0 time.Duration
	if rf, ok := ret.Get(0).(func() time.Duration); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(time.Duration)
	}

	return r0
}

// RemoveStateURI provides a mock function with given fields: stateURI
func (_m *BlobPeerConn) RemoveStateURI(stateURI string) {
	_m.Called(stateURI)
}

// SendBlobHeader provides a mock function with given fields: haveBlob
func (_m *BlobPeerConn) SendBlobHeader(haveBlob bool) error {
	ret := _m.Called(haveBlob)

	var r0 error
	if rf, ok := ret.Get(0).(func(bool) error); ok {
		r0 = rf(haveBlob)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendBlobPacket provides a mock function with given fields: data, end
func (_m *BlobPeerConn) SendBlobPacket(data []byte, end bool) error {
	ret := _m.Called(data, end)

	var r0 error
	if rf, ok := ret.Get(0).(func([]byte, bool) error); ok {
		r0 = rf(data, end)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// StateURIs provides a mock function with given fields:
func (_m *BlobPeerConn) StateURIs() utils.StringSet {
	ret := _m.Called()

	var r0 utils.StringSet
	if rf, ok := ret.Get(0).(func() utils.StringSet); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(utils.StringSet)
		}
	}

	return r0
}

// Transport provides a mock function with given fields:
func (_m *BlobPeerConn) Transport() swarm.Transport {
	ret := _m.Called()

	var r0 swarm.Transport
	if rf, ok := ret.Get(0).(func() swarm.Transport); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(swarm.Transport)
		}
	}

	return r0
}

// UpdateConnStats provides a mock function with given fields: success
func (_m *BlobPeerConn) UpdateConnStats(success bool) {
	_m.Called(success)
}
