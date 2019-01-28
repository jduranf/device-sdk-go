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
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edgexfoundry/device-sdk-go/internal/cache"
	"github.com/edgexfoundry/device-sdk-go/internal/common"
	ds_models "github.com/edgexfoundry/device-sdk-go/pkg/models"
	"github.com/edgexfoundry/edgex-go/pkg/models"
	"github.com/goburrow/modbus"
)

const (
	modbusHoldingRegister = "HoldingRegister"
	modbusInputRegister   = "InputRegister"
	modbusCoil            = "Coil"
)

const (
	modbusTCP   = "TCP"
	modbusHTTP  = "HTTP"
	modbusRTU   = "RTU"
	modbusOTHER = "OTHER"
	comTimeout  = 2000
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
	function   string
	address    uint16
	size       uint16
	vType      string
	isByteSwap bool
	isWordSwap bool
	resultType string
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

func getClient(addressable *models.Addressable) (modbusDevice *ModbusDevice, err error) {
	if addressable.Protocol == modbusTCP || addressable.Protocol == modbusHTTP {
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
	} else if addressable.Protocol == modbusRTU || addressable.Protocol == modbusOTHER {
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

func getTCPConfig(addressable *models.Addressable) (address string, err error) {

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

func getRTUConfig(addressable *models.Addressable) (config rtuConfig, err error) {

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

func getReadValues(do *models.DeviceObject) (readConfig modbusReadConfig, err error) {

	// Get read function
	if len(do.Attributes) < 3 {
		err = fmt.Errorf("Invalid number attributes: %v", do.Attributes)
		return
	}
	readConfig.function = do.Attributes["PrimaryTable"].(string)
	if readConfig.function != "HoldingRegister" && readConfig.function != "InputRegister" && readConfig.function != "Coil" {
		err = fmt.Errorf("Invalid attribute: %v", do.Attributes)
		return
	}
	//	Get address
	strAddress, ok := do.Attributes["StartingAddress"].(string)
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

	// Get number of registers and value Type
	vType := do.Attributes["ValueType"].(string)
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
		nRegs, ok := do.Attributes["Length"].(string)
		if ok == false {
			err = fmt.Errorf("Invalid attribute format: %v", do.Attributes)
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
	isByteSwap := do.Attributes["IsByteSwap"].(string)
	if isByteSwap == "true" || isByteSwap == "True" || isByteSwap == "TRUE" {
		readConfig.isByteSwap = true
	} else {
		readConfig.isByteSwap = false
	}
	isWordSwap := do.Attributes["IsWordSwap"].(string)
	if isWordSwap == "true" || isWordSwap == "True" || isWordSwap == "TRUE" {
		readConfig.isByteSwap = true
	} else {
		readConfig.isByteSwap = false
	}

	if do.Properties.Value.Type == "Bool" || do.Properties.Value.Type == "String" || do.Properties.Value.Type == "Integer" ||
		do.Properties.Value.Type == "Float" || do.Properties.Value.Type == "Json" {
		readConfig.resultType = do.Properties.Value.Type
	} else {
		err = fmt.Errorf("Invalid resultType: %v", do.Properties.Value.Type)
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
	}

	err := fmt.Errorf("Invalid read function: %s", readConfig.function)
	return nil, err
}

func writeModbus(client modbus.Client, readConfig modbusReadConfig, value []byte) ([]byte, error) {
	if readConfig.function == modbusHoldingRegister || readConfig.function == modbusInputRegister {
		return client.WriteMultipleRegisters(readConfig.address, readConfig.size, value)
	} else if readConfig.function == modbusCoil {
		return client.WriteSingleCoil(readConfig.address, binary.LittleEndian.Uint16(value))
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
		myString, _ := param.StringsValue()

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
		data = swapBitDataBytes(param.NumericValue[6:], isByteSwap, isWordSwap)
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

func updateOperatingState(m *ModbusDriver, e error, device *models.Device) {
	var devEn int
	if e != nil {
		if strings.Contains(e.Error(), "timeout") {
			if device.OperatingState == models.Enabled {
				device.OperatingState = models.Disabled
				cache.Devices().Update(*device)
				go common.DeviceClient.UpdateOpStateByName(device.Name, models.Disabled)
				m.lc.Warn(fmt.Sprintf("Updated OperatingState of device: %s to %s", device.Name, models.Disabled))
			}
		}
	} else {
		if device.OperatingState == models.Disabled {
			device.OperatingState = models.Enabled
			cache.Devices().Update(*device)
			go common.DeviceClient.UpdateOpStateByName(device.Name, models.Enabled)
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
