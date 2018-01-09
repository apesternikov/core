// Code generated by mockery v1.0.0
package consumer

import gorocksdb "github.com/tecbot/gorocksdb"
import mock "github.com/stretchr/testify/mock"
import topic "github.com/LiveRamp/gazette/pkg/topic"

// MockShard is an autogenerated mock type for the Shard type
type MockShard struct {
	mock.Mock
}

// Cache provides a mock function with given fields:
func (_m *MockShard) Cache() interface{} {
	ret := _m.Called()

	var r0 interface{}
	if rf, ok := ret.Get(0).(func() interface{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	return r0
}

// Database provides a mock function with given fields:
func (_m *MockShard) Database() *gorocksdb.DB {
	ret := _m.Called()

	var r0 *gorocksdb.DB
	if rf, ok := ret.Get(0).(func() *gorocksdb.DB); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gorocksdb.DB)
		}
	}

	return r0
}

// ID provides a mock function with given fields:
func (_m *MockShard) ID() ShardID {
	ret := _m.Called()

	var r0 ShardID
	if rf, ok := ret.Get(0).(func() ShardID); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(ShardID)
	}

	return r0
}

// Partition provides a mock function with given fields:
func (_m *MockShard) Partition() topic.Partition {
	ret := _m.Called()

	var r0 topic.Partition
	if rf, ok := ret.Get(0).(func() topic.Partition); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(topic.Partition)
	}

	return r0
}

// ReadOptions provides a mock function with given fields:
func (_m *MockShard) ReadOptions() *gorocksdb.ReadOptions {
	ret := _m.Called()

	var r0 *gorocksdb.ReadOptions
	if rf, ok := ret.Get(0).(func() *gorocksdb.ReadOptions); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gorocksdb.ReadOptions)
		}
	}

	return r0
}

// SetCache provides a mock function with given fields: _a0
func (_m *MockShard) SetCache(_a0 interface{}) {
	_m.Called(_a0)
}

// Transaction provides a mock function with given fields:
func (_m *MockShard) Transaction() *gorocksdb.WriteBatch {
	ret := _m.Called()

	var r0 *gorocksdb.WriteBatch
	if rf, ok := ret.Get(0).(func() *gorocksdb.WriteBatch); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gorocksdb.WriteBatch)
		}
	}

	return r0
}

// WriteOptions provides a mock function with given fields:
func (_m *MockShard) WriteOptions() *gorocksdb.WriteOptions {
	ret := _m.Called()

	var r0 *gorocksdb.WriteOptions
	if rf, ok := ret.Get(0).(func() *gorocksdb.WriteOptions); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gorocksdb.WriteOptions)
		}
	}

	return r0
}