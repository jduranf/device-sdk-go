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
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
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

		now := time.Now().UnixNano() / int64(time.Millisecond)
		var result = &ds_models.CommandValue{}

		var valueInt int64
		var valueFloat float64
		var valueBool bool
		var valueString string
		var difType string

		switch readConfig.vType {
		case "UINT16":
			valueInt = int64(binary.BigEndian.Uint16(swapBitDataBytes(data, readConfig.isByteSwap, readConfig.isWordSwap)))
			difType = "Int"
		case "UINT32":
			valueInt = int64(binary.BigEndian.Uint32(swapBitDataBytes(data, readConfig.isByteSwap, readConfig.isWordSwap)))
			difType = "Int"
		case "UINT64":
			valueInt = int64(binary.BigEndian.Uint64(swapBitDataBytes(data, readConfig.isByteSwap, readConfig.isWordSwap)))
			difType = "Int"
		case "INT16":
			valueInt = int64(binary.BigEndian.Uint16(swapBitDataBytes(data, readConfig.isByteSwap, readConfig.isWordSwap)))
			difType = "Int"
		case "INT32":
			valueInt = int64(binary.BigEndian.Uint32(swapBitDataBytes(data, readConfig.isByteSwap, readConfig.isWordSwap)))
			difType = "Int"
		case "INT64":
			valueInt = int64(binary.BigEndian.Uint64(swapBitDataBytes(data, readConfig.isByteSwap, readConfig.isWordSwap)))
			difType = "Int"
		case "FLOAT32":
			valueInt = int64(binary.BigEndian.Uint32(data))
			valueFloat = math.Float64frombits(uint64(valueInt))
			difType = "Float"
		case "FLOAT64":
			valueInt = int64(binary.BigEndian.Uint64(data))
			valueFloat = math.Float64frombits(uint64(valueInt))
			difType = "Float"
		case "BOOL":
			if reqs[i].DeviceObject.Properties.Value.Type == "Bool" {
				difType = "Bool"
				for i := 0; i < len(data); i++ {
					if data[i] == 0 {
						valueBool = false
					} else {
						valueBool = true
						i = len(data)
					}
				}
			} else if reqs[i].DeviceObject.Properties.Value.Type == "String" {
				difType = "String"
				var buf bytes.Buffer
				for _, b := range data {
					fmt.Fprintf(&buf, "%08b ", b)
				}
				buf.Truncate(buf.Len() - 1) // To remove extra space
				valueString = string(buf.Bytes())
			}
		case "STRING":
			//valueString = string(data[:]) // sin filtrar caracteres no printables
			var buffer bytes.Buffer // filtrando caracteres no printables
			for i := 0; i < len(data); i++ {
				if data[i] >= 0x20 && data[i] <= 0x7F {
					valueSt := string(data[i])
					buffer.WriteString(valueSt)
				}
			}
			valueString = buffer.String()
			difType = "String"
		case "ARRAY":
			valueString = hex.EncodeToString(data)
			difType = "String"
		default:
			err = fmt.Errorf("return result fail, none supported value type: %v", reqs[i].DeviceObject.Attributes["ValueType"].(string))
		}

		if reqs[i].DeviceObject.Properties.Value.Type == "Bool" {
			if difType == "Bool" {
				result, err = ds_models.NewBoolValue(&reqs[i].RO, now, valueBool)
			} else if difType == "String" {
				result = ds_models.NewStringValue(&reqs[i].RO, now, valueString)
			}
		} else if reqs[i].DeviceObject.Properties.Value.Type == "String" {
			result = ds_models.NewStringValue(&reqs[i].RO, now, valueString)
		} else if reqs[i].DeviceObject.Properties.Value.Type == "Integer" {
			if difType == "Float" {
				result, err = ds_models.NewInt64Value(&reqs[i].RO, now, int64(valueFloat))
			} else if difType == "Int" {
				result, err = ds_models.NewInt64Value(&reqs[i].RO, now, valueInt)
			}
		} else if reqs[i].DeviceObject.Properties.Value.Type == "Float" {
			if difType == "Float" {
				result, err = ds_models.NewFloat64Value(&reqs[i].RO, now, valueFloat)
			} else if difType == "Int" {
				result, err = ds_models.NewFloat64Value(&reqs[i].RO, now, float64(valueInt))
			}
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
