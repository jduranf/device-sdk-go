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

	ds_models "github.com/edgexfoundry/device-sdk-go/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/clients/logging"
)

type ModbusDriver struct {
	lc      logger.LoggingClient
	asyncCh chan<- *ds_models.AsyncValues
}

const gpioSlavesRedLed = "/sys/class/leds/slaves_red_led/brightness"
const numToDiscover = 7

// DisconnectDevice handles protocol-specific cleanup when a device
// is removed.
func (m *ModbusDriver) DisconnectDevice(deviceName string, protocols map[string]map[string]string) error {
	return nil
}

// Initialize performs protocol-specific initialization for the device
// service.
func (m *ModbusDriver) Initialize(lc logger.LoggingClient, asyncCh chan<- *ds_models.AsyncValues) error {
	m.lc = lc
	m.asyncCh = asyncCh
	initModbusCache()
	m.Discover()
	return nil
}

// HandleReadCommands triggers a protocol Read operation for the specified device.
func (m *ModbusDriver) HandleReadCommands(deviceName string, protocols map[string]map[string]string, reqs []ds_models.CommandRequest) (res []*ds_models.CommandValue, err error) {

	var modbusDevice *ModbusDevice
	modbusDevice, err = getClient(protocols)
	if err != nil {
		m.lc.Warn(fmt.Sprintf("Error connecting with Modbus: %v", err))
		return
	}
	defer releaseClient(modbusDevice)

	res = make([]*ds_models.CommandValue, len(reqs))
	for i := range reqs {
		m.lc.Debug(fmt.Sprintf("ModbusDriver.HandleReadCommands: protocols: %v op: %v attrs: %v", protocols, reqs[i].RO.Operation, reqs[i].DeviceResource.Attributes))

		// TODO: Read multiple registers at the same time if they have contiguous addresses

		var readConfig modbusReadConfig
		readConfig, err = getReadValues(&reqs[i].DeviceResource)
		if err != nil {
			m.lc.Warn(fmt.Sprintf("Error parsing Modbus data: %v", err))
			return
		}

		var data []byte
		numRetries := comRetries
		for {
			data, err = readModbus(modbusDevice.client, readConfig)
			numRetries--
			if (err == nil) || (numRetries == 0) {
				break
			}
		}
		updateOperatingState(m, err, deviceName)
		if err != nil {
			m.lc.Warn(fmt.Sprintf("Error reading Modbus data: %v", err))
			return
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
func (m *ModbusDriver) HandleWriteCommands(deviceName string, protocols map[string]map[string]string, reqs []ds_models.CommandRequest,
	params []*ds_models.CommandValue) error {
	var err error
	var modbusDevice *ModbusDevice
	modbusDevice, err = getClient(protocols)
	if err != nil {
		m.lc.Warn(fmt.Sprintf("Error connecting with Modbus: %v", err))
		return err
	}
	defer releaseClient(modbusDevice)

	for i := range reqs {
		m.lc.Debug(fmt.Sprintf("ModbusDriver.HandleWriteCommands: protocols: %v op: %v attrs: %v", protocols, reqs[i].RO.Operation, reqs[i].DeviceResource.Attributes))

		// TODO: Write multiple registers at the same time if they have contiguous addresses

		var readConfig modbusReadConfig
		readConfig, err = getReadValues(&reqs[i].DeviceResource)
		if err != nil {
			m.lc.Warn(fmt.Sprintf("Error parsing Modbus data: %v", err))
			return err
		}
		var value []byte
		value = setWriteValue(*params[i], readConfig)
		numRetries := comRetries
		for {
			_, err = writeModbus(modbusDevice.client, readConfig, value)
			numRetries--
			if (err == nil) || (numRetries == 0) {
				break
			}
		}
		updateOperatingState(m, err, deviceName)
		if err != nil {
			m.lc.Warn(fmt.Sprintf("Error writing Modbus data: %v", err))
			return err
		}
	}
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

// Discover triggers protocol specific device discovery, which is
// a synchronous operation which returns a list of new devices
// which may be added to the device service based on service
// config. This function may also optionally trigger sensor
// discovery, which could result in dynamic device profile creation.
func (m *ModbusDriver) Discover() error {
	for i := 0; i < numToDiscover; i++ {
		disc, err := discoverScan(2 + i)
		if err != nil {
			m.lc.Error(fmt.Sprintf("ModbusDriver.Discover Error scanning module %v: %v", (2 + i), err))
		} else {
			err = discoverAssign(disc)
			if err != nil {
				m.lc.Error(fmt.Sprintf("ModbusDriver.Discover Error assinging module %v: %v", (2 + i), err))
			}
		}
	}
	return nil
}
