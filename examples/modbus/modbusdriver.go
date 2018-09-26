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
	"strings"
	"time"

	device "github.com/edgexfoundry/device-sdk-go"
	logger "github.com/edgexfoundry/edgex-go/pkg/clients/logging"
	"github.com/edgexfoundry/edgex-go/pkg/models"
	"github.com/goburrow/modbus"
)

type ModbusDevice struct {
	tcpHandler *modbus.TCPClientHandler
	rtuHandler *modbus.RTUClientHandler
	client     modbus.Client
}

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
	return nil
}

// HandleCommand triggers an asynchronous protocol specific GET or SET operation
// for the specified device.
func (m *ModbusDriver) HandleCommands(d models.Device, reqs []device.CommandRequest,
	params string) (res []device.CommandResult, err error) {

	var modbusDevice *ModbusDevice
	modbusDevice, err = m.connectModbus(d.Addressable)
	if err != nil {
		m.lc.Warn(fmt.Sprintf("Error connecting with Modbus: %v", err))
		return
	}

	if modbusDevice.tcpHandler != nil {
		defer modbusDevice.tcpHandler.Close()
	} else if modbusDevice.rtuHandler != nil {
		defer modbusDevice.rtuHandler.Close()
	}

	res = make([]device.CommandResult, len(reqs))
	for i, _ := range reqs {
		m.lc.Debug(fmt.Sprintf("HandleCommand: dev: %s op: %v attrs: %v", d.Name, reqs[i].RO.Operation, reqs[i].DeviceObject.Attributes))

		// TODO: Read multiple registers at the same time if they have contiguous addresses

		var data []byte
		data, err = m.readModbus(modbusDevice.client, &reqs[i].DeviceObject)
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

func (m *ModbusDriver) connectModbus(addressable models.Addressable) (*ModbusDevice, error) {
	var err error
	modbusDevice := &ModbusDevice{}

	// TODO: Make it multithread safe

	if addressable.Protocol == "HTTP" {

		address := fmt.Sprintf("%s:%d", addressable.Address, addressable.Port)
		tcpHandler := modbus.NewTCPClientHandler(address)

		err = tcpHandler.Connect()
		if err != nil {
			err = fmt.Errorf("Couldn't connect: %v", err)
			return modbusDevice, err
		}

		tcpHandler.Timeout = 2000 * time.Millisecond

		modbusDevice.tcpHandler = tcpHandler
		modbusDevice.client = modbus.NewClient(tcpHandler)

	} else if addressable.Protocol == "OTHER" {

		settings := strings.Split(addressable.Address, ",")
		if len(settings) != 5 {
			err = fmt.Errorf("Invalid Modbus RTU address")
			return modbusDevice, err
		}

		rtuHandler := modbus.NewRTUClientHandler(settings[0])

		baudRate, err := strconv.Atoi(settings[1])
		if err != nil {
			err = fmt.Errorf("Invalid baud rate: %v", err)
			return modbusDevice, err
		}
		rtuHandler.BaudRate = baudRate

		if settings[2] != "8" {
			err = fmt.Errorf("Invalid data bits: %s", settings[2])
			return modbusDevice, err
		}
		dataBits, _ := strconv.Atoi(settings[2])
		rtuHandler.DataBits = dataBits

		if settings[3] != "0" && settings[3] != "1" {
			err = fmt.Errorf("Invalid stop bits: %s", settings[3])
			return modbusDevice, err
		}
		stopBits, _ := strconv.Atoi(settings[3])
		rtuHandler.StopBits = stopBits

		if settings[4] == "0" {
			settings[4] = "N"
		} else if settings[4] == "1" {
			settings[4] = "O"
		} else if settings[4] == "2" {
			settings[4] = "E"
		}
		if settings[4] != "N" && settings[4] != "O" && settings[4] != "E" {
			err = fmt.Errorf("Invalid parity: %s", settings[4])
			return modbusDevice, err
		}
		rtuHandler.Parity = settings[4]

		slaveId, err := strconv.Atoi(addressable.Path)
		if err != nil {
			err = fmt.Errorf("Invalid slave ID: %v", err)
			return modbusDevice, err
		}
		rtuHandler.SlaveId = byte(slaveId)

		rtuHandler.Timeout = 2000 * time.Millisecond

		err = rtuHandler.Connect()
		if err != nil {
			err = fmt.Errorf("Couldn't connect: %v", err)
			return modbusDevice, err
		}

		modbusDevice.rtuHandler = rtuHandler
		modbusDevice.client = modbus.NewClient(rtuHandler)

	} else {
		err = fmt.Errorf("Invalid Modbus protocol: %s", addressable.Protocol)
	}

	return modbusDevice, err
}

func (m *ModbusDriver) readModbus(client modbus.Client, do *models.DeviceObject) ([]byte, error) {

	numRegs, err := strconv.Atoi(do.Properties.Value.Size)
	if err != nil {
		err = fmt.Errorf("Invalid number of registers: %v", err)
		return nil, err
	}

	if len(do.Attributes) != 1 {
		err = fmt.Errorf("Invalid number attributes: %v", do.Attributes)
		return nil, err
	}

	var attribute string
	if _, found := do.Attributes["HoldingRegister"]; found {
		attribute = "HoldingRegister"
	} else if _, found := do.Attributes["InputRegister"]; found {
		attribute = "InputRegister"
	} else if _, found := do.Attributes["Coil"]; found {
		attribute = "Coil"
	} else {
		err = fmt.Errorf("Invalid attribute: %v", do.Attributes)
		return nil, err
	}

	strAddress, ok := do.Attributes[attribute].(string)
	if ok == false {
		err = fmt.Errorf("Invalid attribute format: %v", do.Attributes)
		return nil, err
	}
	address, err := strconv.Atoi(strAddress)
	if err != nil {
		err = fmt.Errorf("Invalid address value: %v", err)
		return nil, err
	}

	if attribute == "HoldingRegister" {
		return client.ReadHoldingRegisters(uint16(address), uint16(numRegs))
	} else if attribute == "InputRegister" {
		return client.ReadInputRegisters(uint16(address), uint16(numRegs))
	} else {
		// TODO: Handle read coils results properly
		return client.ReadCoils(uint16(address), uint16(numRegs))
	}
}
