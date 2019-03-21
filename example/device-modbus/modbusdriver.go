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
	"strconv"
	"strings"

	"github.com/edgexfoundry/device-sdk-go/example/device-modbus/comp"
	ds_models "github.com/edgexfoundry/device-sdk-go/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/clients/logging"
)

type ModbusDriver struct {
	lc      logger.LoggingClient
	asyncCh chan<- *ds_models.AsyncValues
}

const gpioSlavesRedLed = "/sys/class/leds/slaves_red_led/brightness"
const ADD_IR_FACT_MODEL = 49804
const ADD_HR_FACT_SERIAL_NUMBER = 61440
const ADD_HR_FACT_SERIAL_ID = 61450
const LEN_IR_FACT_MODEL = 2
const LEN_HR_FACT_SERIAL_NUMBER = 7
const LEN_HR_FACT_SERIAL_ID = 2

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
		data, err = readModbus(modbusDevice.client, readConfig)

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
		_, err = writeModbus(modbusDevice.client, readConfig, value)
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
func (m *ModbusDriver) Discover() (interface{}, error) {
	var err error
	var modbusDevice *ModbusDevice
	var readConf modbusReadConfig
	var data []byte
	var resp [8][2]string

	rtu := map[string]string{
		"Address":  comp.SerialAddress,
		"BaudRate": "115200",
		"DataBits": "8",
		"StopBits": "1",
		"Parity":   "N",
	}
	proto := map[string]map[string]string{
		"ModbusRTU": rtu,
	}

	/*Send query to 8 possibles modules*/
	for i := 0; i < 8; i++ {
		rtu["UnitID"] = strconv.Itoa(i + 1)
		proto["ModbusRTU"] = rtu

		readConf.function = modbusInputRegister
		readConf.address = ADD_IR_FACT_MODEL
		readConf.size = LEN_IR_FACT_MODEL

		modbusDevice, err = getClient(proto)
		if err != nil {
			m.lc.Warn(fmt.Sprintf("Error connecting with Modbus in Discover process: %v", err))
			releaseClient(modbusDevice)
			return resp, err
		}
		data, err = readModbus(modbusDevice.client, readConf)
		if err != nil {
			m.lc.Debug(fmt.Sprintf("Error reading Modbus data in Discover process: %v", err))
			if strings.Contains(err.Error(), "timeout") {
				resp[i][0] = "Empt"
			} else if strings.Contains(err.Error(), "illegal data address") {
				resp[i][0] = "IlAd"
			} else {
				resp[i][0] = " Err"
			}
			resp[i][1] = ""
		} else {
			resp[i][0] = string(data[:])

			readConf.function = modbusHoldingRegister
			readConf.address = ADD_HR_FACT_SERIAL_NUMBER
			readConf.size = LEN_HR_FACT_SERIAL_NUMBER
			data, err = readModbus(modbusDevice.client, readConf)
			if err != nil {
				m.lc.Debug(fmt.Sprintf("Error reading Modbus data in Discover process: %v", err))
				resp[i][1] = "Error"
			} else {
				resp[i][1] = string(data[:])
			}
		}
		releaseClient(modbusDevice)
	}

	return resp, nil
}
