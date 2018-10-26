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
	"sync"
	"time"

	"github.com/edgexfoundry/edgex-go/pkg/models"
	"github.com/goburrow/modbus"
)

const (
	modbusHoldingRegister = "HoldingRegister"
	modbusInputRegister   = "InputRegister"
	modbusCoil            = "Coil"
)

const (
	modbusTCP = "HTTP"
	modbusRTU = "OTHER"
)

type ModbusDevice struct {
	tcpHandler *modbus.TCPClientHandler
	rtuHandler *modbus.RTUClientHandler
	client     modbus.Client
	mutex      sync.Mutex
}

type rtuConfig struct {
	address  string
	baudRate int
	dataBits int
	stopBits int
	parity   string
	slaveID  byte
}

type modbusReadConfig struct {
	function string
	address  uint16
	quantity uint16
}

var (
	initOnce sync.Once
	mMap     map[string]*ModbusDevice
	mapMutex sync.Mutex
)

func initModbusCache() {
	initOnce.Do(func() {
		mMap = make(map[string]*ModbusDevice, 0)
	})
}

func getClient(addressable models.Addressable) (modbusDevice *ModbusDevice, err error) {
	if addressable.Protocol == modbusTCP {
		// Get TCP configuration
		var address string
		address, err = getTCPConfig(addressable)
		if err != nil {
			return
		}

		// If not used before, create TCP device
		mapMutex.Lock()
		if mMap[address] == nil {
			modbusDevice = createTCPDevice(address)
			mMap[address] = modbusDevice
		}
		modbusDevice = mMap[address]
		mapMutex.Unlock()

		// We are going to use the TCP device, lock it
		modbusDevice.mutex.Lock()

		// Connect with TCP device
		err = connectTCPDevice(modbusDevice)
		if err != nil {
			modbusDevice.mutex.Unlock()
			return
		}
	} else if addressable.Protocol == modbusRTU {
		// Get RTU configuration
		var config rtuConfig
		config, err = getRTUConfig(addressable)
		if err != nil {
			return
		}

		// If not used before, create RTU device
		mapMutex.Lock()
		if mMap[config.address] == nil {
			modbusDevice = createRTUDevice(config)
			mMap[config.address] = modbusDevice
		}
		modbusDevice = mMap[config.address]
		mapMutex.Unlock()

		// We are going to use the RTU device, lock it
		modbusDevice.mutex.Lock()

		// Connect with RTU device
		modbusDevice.rtuHandler.SlaveId = config.slaveID
		err = connectRTUDevice(modbusDevice)
		if err != nil {
			modbusDevice.mutex.Unlock()
			return
		}
	} else {
		err = fmt.Errorf("Invalid Modbus protocol: %s", addressable.Protocol)
	}
	return
}

func releaseClient(modbusDevice *ModbusDevice) {
	if modbusDevice.tcpHandler != nil {
		modbusDevice.tcpHandler.Close()
	} else if modbusDevice.rtuHandler != nil {
		modbusDevice.rtuHandler.Close()
	}

	modbusDevice.mutex.Unlock()
}

func getTCPConfig(addressable models.Addressable) (address string, err error) {

	if addressable.Address == "" {
		err = fmt.Errorf("Invalid address")
		return
	}

	if addressable.Port == 0 {
		err = fmt.Errorf("Invalid port")
		return
	}

	address = fmt.Sprintf("%s:%d", addressable.Address, addressable.Port)
	return
}

func getRTUConfig(addressable models.Addressable) (config rtuConfig, err error) {

	settings := strings.Split(addressable.Address, ",")
	if len(settings) != 5 {
		err = fmt.Errorf("Invalid Modbus RTU address")
		return
	}

	config.address = settings[0]

	// Get baudrate
	config.baudRate, err = strconv.Atoi(settings[1])
	if err != nil {
		err = fmt.Errorf("Invalid baud rate: %v", err)
		return
	}

	// Get data bits
	if settings[2] != "8" {
		err = fmt.Errorf("Invalid data bits: %s", settings[2])
		return
	}
	config.dataBits, _ = strconv.Atoi(settings[2])

	// Get stop bits
	if settings[3] != "0" && settings[3] != "1" {
		err = fmt.Errorf("Invalid stop bits: %s", settings[3])
		return
	}
	config.stopBits, _ = strconv.Atoi(settings[3])

	// Get parity
	if settings[4] == "0" {
		settings[4] = "N"
	} else if settings[4] == "1" {
		settings[4] = "O"
	} else if settings[4] == "2" {
		settings[4] = "E"
	}
	if settings[4] != "N" && settings[4] != "O" && settings[4] != "E" {
		err = fmt.Errorf("Invalid parity: %s", settings[4])
		return
	}
	config.parity = settings[4]

	// Get slave ID
	slave, err := strconv.Atoi(addressable.Path)
	if err != nil {
		err = fmt.Errorf("Invalid slave ID: %v", err)
		return
	}
	if (slave == 0) || (slave > 247) {
		err = fmt.Errorf("Invalid slave ID: %d", slave)
		return
	}
	config.slaveID = byte(slave)

	return
}

func createTCPDevice(address string) (modbusDevice *ModbusDevice) {
	modbusDevice = &ModbusDevice{}
	tcpHandler := modbus.NewTCPClientHandler(address)
	tcpHandler.Timeout = 2000 * time.Millisecond
	modbusDevice.tcpHandler = tcpHandler
	modbusDevice.client = modbus.NewClient(tcpHandler)
	return
}

func createRTUDevice(config rtuConfig) (modbusDevice *ModbusDevice) {
	modbusDevice = &ModbusDevice{}
	rtuHandler := modbus.NewRTUClientHandler(config.address)
	rtuHandler.BaudRate = config.baudRate
	rtuHandler.DataBits = config.dataBits
	rtuHandler.StopBits = config.stopBits
	rtuHandler.Parity = config.parity
	rtuHandler.Timeout = 2000 * time.Millisecond
	modbusDevice.rtuHandler = rtuHandler
	modbusDevice.client = modbus.NewClient(rtuHandler)
	return
}

func connectTCPDevice(modbusDevice *ModbusDevice) (err error) {
	err = modbusDevice.tcpHandler.Connect()
	if err != nil {
		err = fmt.Errorf("Couldn't connect: %v", err)
	}
	return
}

func connectRTUDevice(modbusDevice *ModbusDevice) (err error) {
	err = modbusDevice.rtuHandler.Connect()
	if err != nil {
		err = fmt.Errorf("Couldn't connect: %v", err)
	}
	return
}

func getReadValues(do *models.DeviceObject) (readConfig modbusReadConfig, err error) {

	// Get read function
	if len(do.Attributes) != 1 {
		err = fmt.Errorf("Invalid number attributes: %v", do.Attributes)
		return
	}
	if _, found := do.Attributes[modbusHoldingRegister]; found {
		readConfig.function = modbusHoldingRegister
	} else if _, found := do.Attributes[modbusInputRegister]; found {
		readConfig.function = modbusInputRegister
	} else if _, found := do.Attributes[modbusCoil]; found {
		readConfig.function = modbusCoil
	} else {
		err = fmt.Errorf("Invalid attribute: %v", do.Attributes)
		return
	}

	//	Get address
	strAddress, ok := do.Attributes[readConfig.function].(string)
	if ok == false {
		err = fmt.Errorf("Invalid attribute format: %v", do.Attributes)
		return
	}
	var add int
	add, err = strconv.Atoi(strAddress)
	if err != nil {
		err = fmt.Errorf("Invalid address value: %v", err)
		return
	}
	readConfig.address = uint16(add)

	// Get number of registers
	var numRegs int
	numRegs, err = strconv.Atoi(do.Properties.Value.Size)
	if err != nil {
		err = fmt.Errorf("Invalid number of registers: %v", err)
		return
	}
	readConfig.quantity = uint16(numRegs)

	return
}

func readModbus(client modbus.Client, readConfig modbusReadConfig) ([]byte, error) {
	if readConfig.function == modbusHoldingRegister {
		return client.ReadHoldingRegisters(readConfig.address, readConfig.quantity)
	} else if readConfig.function == modbusInputRegister {
		return client.ReadInputRegisters(readConfig.address, readConfig.quantity)
	} else if readConfig.function == modbusCoil {
		return client.ReadCoils(readConfig.address, readConfig.quantity)
	}

	err := fmt.Errorf("Invalid read function: %s", readConfig.function)
	return nil, err
}
