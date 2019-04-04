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
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edgexfoundry/device-sdk-go/example/device-modbus/comp"
	"github.com/edgexfoundry/device-sdk-go/internal/cache"
	"github.com/edgexfoundry/device-sdk-go/internal/common"
	"github.com/edgexfoundry/device-sdk-go/internal/provision"
	ds_models "github.com/edgexfoundry/device-sdk-go/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/models"
	"github.com/goburrow/modbus"
	"github.com/google/uuid"
)

const (
	modbusHoldingRegister = "HoldingRegister"
	modbusInputRegister   = "InputRegister"
	modbusCoil            = "Coil"
	modbusDiscreteInput   = "DiscreteInput"
)

const (
	comTimeout = 2000
)

const addIrFactModel = 49804
const addHrFactSerialNumber = 61440
const lenIrFactModel = 2
const lenHrFactSerialNumber = 7

const maxPrecision = 6

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
	function   string
	address    uint16
	size       uint16
	vType      string
	isByteSwap bool
	isWordSwap bool
	resultType string
	precision  int
}

type discoverResult struct {
	protocols   map[string]map[string]string
	identifiers map[string]string
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

func getClient(protocols map[string]map[string]string) (modbusDevice *ModbusDevice, err error) {
	modbusTCP, okTCP := protocols["ModbusTCP"]
	modbusRTU, okRTU := protocols["ModbusRTU"]
	if okTCP {
		// Get TCP configuration
		var address string
		address, err = getTCPConfig(modbusTCP)
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
	} else if okRTU {
		// Get RTU configuration
		var config rtuConfig
		config, err = getRTUConfig(modbusRTU)
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
		modbusDevice.rtuHandler.BaudRate = config.baudRate
		modbusDevice.rtuHandler.DataBits = config.dataBits
		modbusDevice.rtuHandler.StopBits = config.stopBits
		modbusDevice.rtuHandler.Parity = config.parity
		modbusDevice.rtuHandler.SlaveId = config.slaveID
		err = connectRTUDevice(modbusDevice)
		if err != nil {
			modbusDevice.mutex.Unlock()
			return
		}
	} else {
		err = fmt.Errorf("Invalid Modbus protocol: %v", protocols)
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

func getTCPConfig(protocol map[string]string) (url string, err error) {

	host, ok := protocol["Host"]
	if !ok {
		err = fmt.Errorf("Host not defined")
		return
	}
	if host == "" {
		err = fmt.Errorf("Invalid host")
		return
	}

	portStr, ok := protocol["Port"]
	if !ok {
		err = fmt.Errorf("Port not defined")
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		err = fmt.Errorf("Invalid port: %v", err)
		return
	}
	if port == 0 {
		err = fmt.Errorf("Invalid port value: %d", port)
		return
	}

	url = fmt.Sprintf("%s:%d", host, port)
	return
}

func getRTUConfig(protocol map[string]string) (config rtuConfig, err error) {

	// Get serial port
	address, ok := protocol["Address"]
	if !ok {
		err = fmt.Errorf("Address not defined")
		return
	}
	config.address = address

	// Get baudrate
	baudRate, ok := protocol["BaudRate"]
	if !ok {
		err = fmt.Errorf("Baud rate not defined")
		return
	}
	config.baudRate, err = strconv.Atoi(baudRate)
	if err != nil {
		err = fmt.Errorf("Invalid baud rate: %v", err)
		return
	}

	// Get data bits
	dataBits, ok := protocol["DataBits"]
	if !ok {
		err = fmt.Errorf("Data bits not defined")
		return
	}
	if dataBits != "8" {
		err = fmt.Errorf("Invalid data bits value: %s", dataBits)
		return
	}
	config.dataBits = 8

	// Get stop bits
	stopBits, ok := protocol["StopBits"]
	if !ok {
		err = fmt.Errorf("Stop bits not defined")
		return
	}
	if stopBits != "0" && stopBits != "1" {
		err = fmt.Errorf("Invalid stop bits: %s", stopBits)
		return
	}
	config.stopBits, _ = strconv.Atoi(stopBits)

	// Get parity
	parity, ok := protocol["Parity"]
	if !ok {
		err = fmt.Errorf("Stop bits not defined")
		return
	}
	if parity == "0" {
		parity = "N"
	} else if parity == "1" {
		parity = "O"
	} else if parity == "2" {
		parity = "E"
	}
	if parity != "N" && parity != "O" && parity != "E" {
		err = fmt.Errorf("Invalid parity: %s", parity)
		return
	}
	config.parity = parity

	// Get unit ID
	unitID, ok := protocol["UnitID"]
	if !ok {
		err = fmt.Errorf("Unit ID not defined")
		return
	}
	unit, err := strconv.Atoi(unitID)
	if err != nil {
		err = fmt.Errorf("Invalid unit ID value: %v", err)
		return
	}
	if (unit == 0) || (unit > 247) {
		err = fmt.Errorf("Invalid unit ID value: %d", unit)
		return
	}
	config.slaveID = byte(unit)

	return
}

func createTCPDevice(address string) (modbusDevice *ModbusDevice) {
	modbusDevice = &ModbusDevice{}
	tcpHandler := modbus.NewTCPClientHandler(address)
	tcpHandler.Timeout = comTimeout * time.Millisecond
	modbusDevice.tcpHandler = tcpHandler
	modbusDevice.client = modbus.NewClient(tcpHandler)
	return
}

func createRTUDevice(config rtuConfig) (modbusDevice *ModbusDevice) {
	modbusDevice = &ModbusDevice{}
	rtuHandler := modbus.NewRTUClientHandler(config.address)
	rtuHandler.Timeout = comTimeout * time.Millisecond
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

func getReadValues(dr *models.DeviceResource) (readConfig modbusReadConfig, err error) {

	// Get read function
	if len(dr.Attributes) < 3 {
		err = fmt.Errorf("Invalid number attributes: %v", dr.Attributes)
		return
	}
	readConfig.function = dr.Attributes["PrimaryTable"].(string)
	if readConfig.function != "HoldingRegister" && readConfig.function != "InputRegister" && readConfig.function != "Coil" && readConfig.function != "DiscreteInput" {
		err = fmt.Errorf("Invalid attribute: %v", dr.Attributes)
		return
	}
	//	Get address
	strAddress, ok := dr.Attributes["StartingAddress"].(string)
	if ok == false {
		err = fmt.Errorf("Invalid attribute format: %v", dr.Attributes)
		return
	}
	var add int
	add, err = strconv.Atoi(strAddress)
	if err != nil {
		err = fmt.Errorf("Invalid address value: %v", err)
		return
	}
	readConfig.address = uint16(add)

	// Get number of registers and value Type
	vType := dr.Attributes["ValueType"].(string)
	if vType == "UINT8" || vType == "INT8" || vType == "UINT16" || vType == "INT16" || vType == "FLOAT16" || vType == "BOOL" {
		readConfig.size = 1
		readConfig.vType = vType
	} else if vType == "UINT32" || vType == "INT32" || vType == "FLOAT32" {
		readConfig.size = 2
		readConfig.vType = vType
	} else if vType == "UINT64" || vType == "INT64" || vType == "FLOAT64" {
		readConfig.size = 4
		readConfig.vType = vType
	} else if vType == "STRING" || vType == "ARRAY" {
		readConfig.vType = vType
		nRegs, ok := dr.Attributes["Length"].(string)
		if ok == false {
			err = fmt.Errorf("Invalid attribute format: %v", dr.Attributes)
			return
		}
		var reg int
		reg, err = strconv.Atoi(nRegs)
		if err != nil {
			err = fmt.Errorf("Invalid Length value: %v", err)
			return
		}
		readConfig.size = uint16(reg)
	} else {
		err = fmt.Errorf("Invalid ValueType: %v", err)
		return
	}

	// Get Swap
	isByteSwap := dr.Attributes["IsByteSwap"].(string)
	if isByteSwap == "true" || isByteSwap == "True" || isByteSwap == "TRUE" {
		readConfig.isByteSwap = true
	} else {
		readConfig.isByteSwap = false
	}
	isWordSwap := dr.Attributes["IsWordSwap"].(string)
	if isWordSwap == "true" || isWordSwap == "True" || isWordSwap == "TRUE" {
		readConfig.isByteSwap = true
	} else {
		readConfig.isByteSwap = false
	}

	if dr.Properties.Value.Type == "Bool" || dr.Properties.Value.Type == "String" || dr.Properties.Value.Type == "Integer" ||
		dr.Properties.Value.Type == "Float" || dr.Properties.Value.Type == "Json" {
		readConfig.resultType = dr.Properties.Value.Type

		if dr.Properties.Value.Type == "Float" {
			if dr.Properties.Value.Precision != "" {
				readConfig.precision, err = strconv.Atoi(dr.Properties.Value.Precision)
				if err != nil {
					readConfig.precision = maxPrecision
				}
			} else {
				readConfig.precision = maxPrecision
			}
		}
	} else {
		err = fmt.Errorf("Invalid resultType: %v", dr.Properties.Value.Type)
		return
	}
	return
}

func readModbus(client modbus.Client, readConfig modbusReadConfig) ([]byte, error) {
	if readConfig.function == modbusHoldingRegister {
		return client.ReadHoldingRegisters(readConfig.address, readConfig.size)
	} else if readConfig.function == modbusInputRegister {
		return client.ReadInputRegisters(readConfig.address, readConfig.size)
	} else if readConfig.function == modbusCoil {
		return client.ReadCoils(readConfig.address, readConfig.size)
	} else if readConfig.function == modbusDiscreteInput {
		return client.ReadDiscreteInputs(readConfig.address, readConfig.size)
	}

	err := fmt.Errorf("Invalid read function: %s", readConfig.function)
	return nil, err
}

func writeModbus(client modbus.Client, readConfig modbusReadConfig, value []byte) ([]byte, error) {
	if readConfig.function == modbusHoldingRegister || readConfig.function == modbusInputRegister {
		return client.WriteMultipleRegisters(readConfig.address, readConfig.size, value)
	} else if readConfig.function == modbusCoil {
		var val uint16
		if uint16(value[0]) != 0 {
			val = 0xFF00
		} else {
			val = 0
		}
		return client.WriteSingleCoil(readConfig.address, val)
	}

	err := fmt.Errorf("Invalid write function: %s", readConfig.function)
	return nil, err
}

func setResult(readConf modbusReadConfig, dat []byte, creq ds_models.CommandRequest) (ds_models.CommandValue, error) {
	var valueInt int64
	var valueFloat float64
	var valueBool bool
	var valueString string
	difType := "difType"
	var result = &ds_models.CommandValue{}

	switch readConf.vType {
	case "UINT16":
		valueInt = int64(binary.BigEndian.Uint16(swapBitDataBytes(dat, readConf.isByteSwap, readConf.isWordSwap)))
		difType = "Int"
	case "UINT32":
		valueInt = int64(binary.BigEndian.Uint32(swapBitDataBytes(dat, readConf.isByteSwap, readConf.isWordSwap)))
		difType = "Int"
	case "UINT64":
		valueInt = int64(binary.BigEndian.Uint64(swapBitDataBytes(dat, readConf.isByteSwap, readConf.isWordSwap)))
		difType = "Int"
	case "INT16":
		valueInt = int64(binary.BigEndian.Uint16(swapBitDataBytes(dat, readConf.isByteSwap, readConf.isWordSwap)))
		difType = "Int"
	case "INT32":
		valueInt = int64(binary.BigEndian.Uint32(swapBitDataBytes(dat, readConf.isByteSwap, readConf.isWordSwap)))
		difType = "Int"
	case "INT64":
		valueInt = int64(binary.BigEndian.Uint64(swapBitDataBytes(dat, readConf.isByteSwap, readConf.isWordSwap)))
		difType = "Int"
	case "FLOAT32":
		valueFloat = float64(math.Float32frombits(binary.BigEndian.Uint32(dat)))
		difType = "Float"
	case "FLOAT64":
		valueFloat = math.Float64frombits(binary.BigEndian.Uint64(dat))
		difType = "Float"
	case "BOOL":
		difType = "Bool"
		for i := 0; i < len(dat); i++ {
			if dat[i] == 0 {
				valueBool = false
			} else {
				valueBool = true
				break
			}
		}
	case "STRING":
		var buffer bytes.Buffer
		for i := 0; i < len(dat); i++ {
			if dat[i] >= 0x20 && dat[i] <= 0x7F {
				valueSt := string(dat[i])
				buffer.WriteString(valueSt)
			}
		}
		valueString = buffer.String()
		difType = "String"
	case "ARRAY":
		valueString = hex.EncodeToString(dat)
		difType = "String"
	default:
		err := fmt.Errorf("return result fail, none supported value type: %v", readConf.vType)
		return *result, err
	}

	var err error
	now := time.Now().UnixNano() / int64(time.Millisecond)
	if readConf.resultType == "Bool" {
		if difType == "Bool" {
			result, err = ds_models.NewBoolValue(&creq.RO, now, valueBool)
		} else if difType == "String" {
			result = ds_models.NewStringValue(&creq.RO, now, valueString)
		}
	} else if readConf.resultType == "String" {
		result = ds_models.NewStringValue(&creq.RO, now, valueString)
	} else if readConf.resultType == "Integer" {
		if difType == "Float" {
			result, err = ds_models.NewInt64Value(&creq.RO, now, int64(valueFloat))
		} else if difType == "Int" {
			result, err = ds_models.NewInt64Value(&creq.RO, now, valueInt)
		}
	} else if readConf.resultType == "Float" {
		if difType == "Float" {
			output := math.Pow(10, float64(readConf.precision))
			valueFloat = float64(math.Round(valueFloat*output)) / output
			result, err = ds_models.NewFloat64Value(&creq.RO, now, valueFloat)
		} else if difType == "Int" {
			result, err = ds_models.NewFloat64Value(&creq.RO, now, float64(valueInt))
		}
	}

	return *result, err
}

func setWriteValue(param ds_models.CommandValue, writeConf modbusReadConfig) []byte {
	var data []byte
	var isByteSwap bool
	var isWordSwap bool
	var i uint16

	if writeConf.resultType == "String" {
		myString, _ := param.StringValue()

		if writeConf.vType == "STRING" {
			if len(myString) == int(writeConf.size) {
				for i = 0; i < writeConf.size; i++ {
					data = append(data, byte(0))
					data = append(data, byte(myString[i]))
				}
			} else {
				data = []byte(myString)
			}
		}
		if writeConf.vType == "ARRAY" {
			if len(myString) == int(writeConf.size*2) {
				datastring, _ := hex.DecodeString(myString)
				for i = 0; i < writeConf.size; i++ {
					data = append(data, byte(0))
					data = append(data, datastring[i])
				}
			} else {
				data, _ = hex.DecodeString(myString)
			}
		}
	} else if writeConf.resultType == "Bool" {
		data = param.NumericValue
	} else if writeConf.resultType == "Json" {
		//TODO:JSon case
	} else if writeConf.resultType == "Integer" {
		switch writeConf.vType {
		case "UINT8":
			data = param.NumericValue[7:]
		case "INT8":
			data = param.NumericValue[7:]
		case "UINT16":
			data = swapBitDataBytes(param.NumericValue[6:], isByteSwap, isWordSwap)
		case "INT16":
			data = swapBitDataBytes(param.NumericValue[6:], isByteSwap, isWordSwap)
		case "UINT32":
			data = swapBitDataBytes(param.NumericValue[4:], isByteSwap, isWordSwap)
		case "INT32":
			data = swapBitDataBytes(param.NumericValue[4:], isByteSwap, isWordSwap)
		case "UINT64":
			data = swapBitDataBytes(param.NumericValue, isByteSwap, isWordSwap)
		case "INT64":
			data = swapBitDataBytes(param.NumericValue, isByteSwap, isWordSwap)

		case "FLOAT32":
			data = swapBitDataBytes(param.NumericValue[4:], isByteSwap, isWordSwap)
		case "FLOAT64":
			data = swapBitDataBytes(param.NumericValue, isByteSwap, isWordSwap)
		}

	} else if writeConf.resultType == "Float" {
		if writeConf.vType == "FLOAT64" {
			dat := math.Float64frombits(binary.BigEndian.Uint64(param.NumericValue))
			binary.BigEndian.PutUint64(data, math.Float64bits(dat))
		} else if writeConf.vType == "FLOAT32" {
			dat := math.Float32frombits(binary.BigEndian.Uint32(param.NumericValue))
			binary.BigEndian.PutUint32(data, math.Float32bits(dat))
		} else {
			dat := math.Float64frombits(binary.BigEndian.Uint64(param.NumericValue))
			binary.BigEndian.PutUint64(param.NumericValue, uint64(dat))
			switch writeConf.vType {
			case "UINT8":
				data = param.NumericValue[7:]
			case "INT8":
				data = param.NumericValue[7:]
			case "UINT16":
				data = swapBitDataBytes(param.NumericValue[6:], isByteSwap, isWordSwap)
			case "INT16":
				data = swapBitDataBytes(param.NumericValue[6:], isByteSwap, isWordSwap)
			case "UINT32":
				data = swapBitDataBytes(param.NumericValue[4:], isByteSwap, isWordSwap)
			case "INT32":
				data = swapBitDataBytes(param.NumericValue[4:], isByteSwap, isWordSwap)
			case "UINT64":
				data = swapBitDataBytes(param.NumericValue, isByteSwap, isWordSwap)
			case "INT64":
				data = swapBitDataBytes(param.NumericValue, isByteSwap, isWordSwap)
			}
		}
	}

	return data
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

func updateOperatingState(m *ModbusDriver, e error, deviceName string) {
	var devEn int

	device, _ := cache.Devices().ForName(deviceName)
	ctx := context.WithValue(context.Background(), common.CorrelationHeader, uuid.New().String())
	if e != nil {
		if strings.Contains(e.Error(), "timeout") {
			if device.OperatingState == models.Enabled {
				device.OperatingState = models.Disabled
				cache.Devices().Update(device)
				go common.DeviceClient.UpdateOpStateByName(device.Name, models.Disabled, ctx)
				m.lc.Warn(fmt.Sprintf("Updated OperatingState of device: %s to %s", device.Name, models.Disabled))
			}
		}
	} else {
		if device.OperatingState == models.Disabled {
			device.OperatingState = models.Enabled
			cache.Devices().Update(device)
			go common.DeviceClient.UpdateOpStateByName(device.Name, models.Enabled, ctx)
			m.lc.Info(fmt.Sprintf("Updated OperatingState of device: %s to %s", device.Name, models.Enabled))
		}
	}
	devEn = 0
	allDevices := cache.Devices().All()
	for i := range allDevices {
		if allDevices[i].OperatingState == models.Enabled {
			devEn++
		}
	}
	if devEn == len(allDevices) {
		ioutil.WriteFile(gpioSlavesRedLed, []byte("0"), 0644)
	} else {
		ioutil.WriteFile(gpioSlavesRedLed, []byte("1"), 0644)
	}
}

func discoverScan(address int) (discoverResult, error) {
	var disc discoverResult

	dev := map[string]string{}
	rtu := map[string]string{
		"Address":  comp.SerialAddress,
		"BaudRate": "115200",
		"DataBits": "8",
		"StopBits": "1",
		"Parity":   "N",
		"UnitID":   strconv.Itoa(address),
	}
	disc.protocols = map[string]map[string]string{
		"ModbusRTU": rtu,
	}

	modbusDevice, err := getClient(disc.protocols)
	if err != nil {
		releaseClient(modbusDevice)
		return disc, err
	}

	// Get device model
	var readConf modbusReadConfig
	readConf.function = modbusInputRegister
	readConf.address = addIrFactModel
	readConf.size = lenIrFactModel
	var data []byte
	data, err = readModbus(modbusDevice.client, readConf)
	if err != nil {
		releaseClient(modbusDevice)
		return disc, err
	}
	dev["Model"] = string(data[:])

	// Get device serial number
	readConf.function = modbusHoldingRegister
	readConf.address = addHrFactSerialNumber
	readConf.size = lenHrFactSerialNumber
	data, err = readModbus(modbusDevice.client, readConf)
	if err != nil {
		releaseClient(modbusDevice)
		return disc, err
	}
	dev["SerialNum"] = string(data[:])

	releaseClient(modbusDevice)

	disc.identifiers = dev

	return disc, nil
}

func discoverAssign(disc discoverResult) error {
	// Check if device already exist
	nameDevice := disc.identifiers["Model"] + "_SN:" + disc.identifiers["SerialNum"]
	_, ok := cache.Devices().ForName(nameDevice)
	if ok {
		return nil
	}

	// Search provision watcher
	pw, ok := cache.Watchers().ForName(disc.identifiers["Model"])
	if !ok {
		errMsg := fmt.Sprintf("ProvisionWatcher %s doesn't exist for Device %s", disc.identifiers["Model"], nameDevice)
		return fmt.Errorf(errMsg)
	}

	// Add device
	var deviceConf common.DeviceConfig
	deviceConf.Name = nameDevice
	deviceConf.Profile = pw.Profile.Name
	deviceConf.Protocols = disc.protocols
	err := provision.CreateDevice(deviceConf)
	if err != nil {
		errMsg := fmt.Sprintf("creating Device %s failed", nameDevice)
		return fmt.Errorf(errMsg)
	}

	return nil
}
