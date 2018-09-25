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
	"strconv"
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
	m.lc.Debug(fmt.Sprintf("SimpleHandler.Initialize called!"))
	return nil
}

// HandleCommand triggers an asynchronous protocol specific GET or SET operation
// for the specified device.
func (m *ModbusDriver) HandleCommands(d models.Device, reqs []device.CommandRequest,
	params string) (res []device.CommandResult, err error) {

	var requestResult RequestResult

	commconfig := d.Addressable.Address
	ch := make(chan RequestResult, 10)

	err, engineModbus := Create(commconfig, ch)
	if err != nil {
		m.lc.Debug(fmt.Sprintf("ModbusHandler.Error connecting Modbus: %v", err))
		return
	}

	res = make([]device.CommandResult, len(reqs))

	for i, _ := range reqs {
		m.lc.Debug(fmt.Sprintf("HandleCommand: dev: %s op: %v attrs: %v", d.Name, reqs[i].RO.Operation, reqs[i].DeviceObject.Attributes))

		// Fill requestModbus
		var requestModbus RequestModbus

		address, _ := strconv.Atoi(reqs[i].DeviceObject.Attributes["HoldingRegister"].(string))
		requestModbus.Address = uint16(address)
		size, _ := strconv.Atoi(reqs[i].DeviceObject.Properties.Value.Size)
		requestModbus.NumRegs = uint16(size)
		slaveId, _ := strconv.Atoi(d.Addressable.Path)
		requestModbus.SlaveId = byte(slaveId)

		requestModbus.Retry = 1

		for index := 0; index < requestModbus.Retry; index++ {
			engineModbus.AddRequest(requestModbus)
			requestResult = engineModbus.LaunchUnit()

			if requestResult.Err == nil {
				index = requestModbus.Retry
			}
		}

		res[i].DeviceName = d.Name
		res[i].DeviceId = d.Id.Hex()
		res[i].RO = &reqs[i].RO
		res[i].Origin = time.Now().UnixNano() / int64(time.Millisecond)
		res[i].Type = device.Uint16
		res[i].NumericResult = requestResult.Data
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
