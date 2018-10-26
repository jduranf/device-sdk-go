// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Circutor S.A.
//
// SPDX-License-Identifier: Apache-2.0
//
// This package provides a Modbus implementation of
// a ProtocolDriver interface.
//
package modbus

import (
	"fmt"
	"time"

	device "github.com/edgexfoundry/device-sdk-go"
	logger "github.com/edgexfoundry/edgex-go/pkg/clients/logging"
	"github.com/edgexfoundry/edgex-go/pkg/models"
)

type ModbusDriver struct {
	lc logger.LoggingClient
}

// DisconnectDevice handles protocol-specific cleanup when a device
// is removed.
func (m *ModbusDriver) DisconnectDevice(address *models.Addressable) error {
	return nil
}

// Initialize performs protocol-specific initialization for the device
// service.  If the DS supports asynchronous data pushed from devices/sensors,
// then a valid receive' channel must be created and returned, otherwise nil
// is returned.
func (m *ModbusDriver) Initialize(svc *device.Service, lc logger.LoggingClient, asyncCh <-chan *device.CommandResult) error {
	m.lc = lc
	m.lc.Debug(fmt.Sprintf("ModbusHandler.Initialize called!"))
	initModbusCache()
	return nil
}

// HandleCommands triggers an asynchronous protocol specific GET or SET operation
// for the specified device.
func (m *ModbusDriver) HandleCommands(d models.Device, reqs []device.CommandRequest,
	params string) (res []device.CommandResult, err error) {

	var modbusDevice *ModbusDevice
	modbusDevice, err = getClient(d.Addressable)
	if err != nil {
		m.lc.Warn(fmt.Sprintf("Error connecting with Modbus: %v", err))
		return
	}
	defer releaseClient(modbusDevice)

	res = make([]device.CommandResult, len(reqs))
	for i := range reqs {
		m.lc.Debug(fmt.Sprintf("HandleCommand: dev: %s op: %v attrs: %v", d.Name, reqs[i].RO.Operation, reqs[i].DeviceObject.Attributes))

		// TODO: Read multiple registers at the same time if they have contiguous addresses

		var readConfig modbusReadConfig
		readConfig, err = getReadValues(&reqs[i].DeviceObject)
		if err != nil {
			m.lc.Warn(fmt.Sprintf("Error parsing Modbus data: %v", err))
			return
		}

		var data []byte
		data, err = readModbus(modbusDevice.client, readConfig)
		if err != nil {
			m.lc.Warn(fmt.Sprintf("Error reading Modbus data: %v", err))
			return
		}

		res[i].DeviceName = d.Name
		res[i].DeviceId = d.Id.Hex()
		res[i].RO = &reqs[i].RO
		res[i].Origin = time.Now().UnixNano() / int64(time.Millisecond)
		res[i].Type = device.Uint16
		res[i].NumericResult = data
	}

	return
}

// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (m *ModbusDriver) Stop(force bool) error {
	m.lc.Debug(fmt.Sprintf("Stop called: force=%v", force))
	return nil
}
