// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Circutor S.A.
//
// SPDX-License-Identifier: Apache-2.0

// This package provides a Modbus implementation of
// a ProtocolDriver interface.
//
package modbus

import (
	"fmt"

	"github.com/edgexfoundry/device-sdk-go/internal/cache"
	"github.com/edgexfoundry/device-sdk-go/internal/common"
	ds_models "github.com/edgexfoundry/device-sdk-go/pkg/models"
	"github.com/edgexfoundry/edgex-go/pkg/clients/logging"
	"github.com/edgexfoundry/edgex-go/pkg/models"
	"github.com/goburrow/serial"
)

type ModbusDriver struct {
	lc      logger.LoggingClient
	asyncCh chan<- *ds_models.AsyncValues
}

// DisconnectDevice handles protocol-specific cleanup when a device
// is removed.
func (m *ModbusDriver) DisconnectDevice(address *models.Addressable) error {
	return nil
}

// Initialize performs protocol-specific initialization for the device
// service.
func (m *ModbusDriver) Initialize(lc logger.LoggingClient, asyncCh chan<- *ds_models.AsyncValues) error {
	m.lc = lc
	m.asyncCh = asyncCh
	initModbusCache()
	return nil
}

// HandleReadCommands triggers a protocol Read operation for the specified device.
func (m *ModbusDriver) HandleReadCommands(dev *models.Device, addr *models.Addressable, reqs []ds_models.CommandRequest) (res []*ds_models.CommandValue, err error) {

	var modbusDevice *ModbusDevice
	modbusDevice, err = getClient(addr)
	if err != nil {
		m.lc.Warn(fmt.Sprintf("Error connecting with Modbus: %v", err))
		return
	}
	defer releaseClient(modbusDevice)

	res = make([]*ds_models.CommandValue, len(reqs))
	for i := range reqs {
		m.lc.Debug(fmt.Sprintf("ModbusDriver.HandleReadCommands: dev: %s op: %v attrs: %v", addr.Name, reqs[i].RO.Operation, reqs[i].DeviceObject.Attributes))

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
			//TODO:Add error cases
			if err == serial.ErrTimeout || err.Error() == "server device failure" || err.Error() == "acknowledge" || err.Error() == "unknown" {
				if dev.OperatingState == models.Enabled {
					dev.OperatingState = models.Disabled
					cache.Devices().Update(*dev)
					go common.DeviceClient.UpdateOpStateByName(dev.Name, models.Disabled)
					m.lc.Warn(fmt.Sprintf("Updated OperatingState of device: %s to %s", dev.Name, models.Disabled))
				}
			}
			m.lc.Warn(fmt.Sprintf("Error reading Modbus data: %v", err))
			return
		} else {
			if dev.OperatingState == models.Disabled {
				dev.OperatingState = models.Enabled
				cache.Devices().Update(*dev)
				go common.DeviceClient.UpdateOpStateByName(dev.Name, models.Enabled)
				m.lc.Info(fmt.Sprintf("Updated OperatingState of device: %s to %s", dev.Name, models.Enabled))
			}

		}

		var result = &ds_models.CommandValue{}
		*result, err = setResult(readConfig, data, reqs[i])
		if err != nil {
			m.lc.Warn(fmt.Sprintf("Error setting result Modbus data: %v", err))
			return
		}

		res[i] = result
	}
	return
}

// HandleWriteCommands passes a slice of CommandRequest struct each representing
// a ResourceOperation for a specific device resource (aka DeviceObject).
// Since the commands are actuation commands, params provide parameters for the individual
// command.
func (m *ModbusDriver) HandleWriteCommands(addr *models.Addressable, reqs []ds_models.CommandRequest,
	params []*ds_models.CommandValue) error {

	err := fmt.Errorf("ModbusDriver.HandleWriteCommands not implemented")
	return err
}

// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (m *ModbusDriver) Stop(force bool) error {
	m.lc.Debug(fmt.Sprintf("ModbusDriver.Stop called: force=%v", force))
	return nil
}
