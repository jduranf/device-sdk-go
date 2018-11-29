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
	"time"

	ds_models "github.com/edgexfoundry/device-sdk-go/pkg/models"
	"github.com/edgexfoundry/edgex-go/pkg/clients/logging"
	"github.com/edgexfoundry/edgex-go/pkg/models"
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
func (m *ModbusDriver) HandleReadCommands(addr *models.Addressable, reqs []ds_models.CommandRequest) (res []*ds_models.CommandValue, err error) {

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
			m.lc.Warn(fmt.Sprintf("Error reading Modbus data: %v", err))
			return
		}

		var valueInt int64
		var valueFloat float64
		var valueBool bool
		var valueString string
		difType := "difType"
		err = readResult(readConfig, data, &difType, &valueInt, &valueFloat, &valueBool, &valueString)
		if err != nil {
			m.lc.Warn(fmt.Sprintf("Error reading Modbus data: %v", err))
			return
		}

		//res[i] = setResult (result reqs []ds_models.CommandRequest)

		now := time.Now().UnixNano() / int64(time.Millisecond)
		var tresult = &ds_models.CommandValue{}

		if readConfig.resultType == "Bool" {
			if difType == "Bool" {

				tresult, err = ds_models.NewBoolValue(&reqs[i].RO, now, valueBool)
			} else if difType == "String" {
				tresult = ds_models.NewStringValue(&reqs[i].RO, now, valueString)
			}
		} else if readConfig.resultType == "String" {
			tresult = ds_models.NewStringValue(&reqs[i].RO, now, valueString)
		} else if readConfig.resultType == "Integer" {
			if difType == "Float" {
				tresult, err = ds_models.NewInt64Value(&reqs[i].RO, now, int64(valueFloat))
			} else if difType == "Int" {
				tresult, err = ds_models.NewInt64Value(&reqs[i].RO, now, valueInt)
			}
		} else if readConfig.resultType == "Float" {
			if difType == "Float" {
				tresult, err = ds_models.NewFloat64Value(&reqs[i].RO, now, valueFloat)
			} else if difType == "Int" {
				tresult, err = ds_models.NewFloat64Value(&reqs[i].RO, now, float64(valueInt))
			}
		}
		res[i] = tresult
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

func swapBitDataBytes(dataBytes []byte, isByteSwap bool, isWordSwap bool) []byte {

	if !isByteSwap && !isWordSwap {
		return dataBytes
	}

	if len(dataBytes) == 2 {
		var newDataBytes = make([]byte, len(dataBytes))
		if isByteSwap {
			newDataBytes[0] = dataBytes[1]
			newDataBytes[1] = dataBytes[0]
		}
		return newDataBytes
	}
	if len(dataBytes) == 4 {
		var newDataBytes = make([]byte, len(dataBytes))

		if isByteSwap {
			newDataBytes[0] = dataBytes[1]
			newDataBytes[1] = dataBytes[0]
			newDataBytes[2] = dataBytes[3]
			newDataBytes[3] = dataBytes[2]
		}
		if isWordSwap {
			newDataBytes[0] = dataBytes[2]
			newDataBytes[1] = dataBytes[3]
			newDataBytes[2] = dataBytes[0]
			newDataBytes[3] = dataBytes[1]
		}
		return newDataBytes
	}
	if len(dataBytes) == 8 {
		var newDataBytes = make([]byte, len(dataBytes))

		if isByteSwap {
			newDataBytes[0] = dataBytes[1]
			newDataBytes[1] = dataBytes[0]
			newDataBytes[2] = dataBytes[3]
			newDataBytes[3] = dataBytes[2]
			newDataBytes[4] = dataBytes[5]
			newDataBytes[5] = dataBytes[4]
			newDataBytes[6] = dataBytes[7]
			newDataBytes[7] = dataBytes[6]

		}
		if isWordSwap {
			newDataBytes[0] = dataBytes[6]
			newDataBytes[1] = dataBytes[7]
			newDataBytes[2] = dataBytes[4]
			newDataBytes[3] = dataBytes[5]
			newDataBytes[4] = dataBytes[2]
			newDataBytes[5] = dataBytes[3]
			newDataBytes[6] = dataBytes[0]
			newDataBytes[7] = dataBytes[1]
		}
		return newDataBytes
	}

	return dataBytes
}
