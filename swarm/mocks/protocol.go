// Code generated by mockery v2.8.0. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// Protocol is an autogenerated mock type for the Protocol type
type Protocol struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *Protocol) Close() {
	_m.Called()
}

// Name provides a mock function with given fields:
func (_m *Protocol) Name() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Start provides a mock function with given fields:
func (_m *Protocol) Start() {
	_m.Called()
}
