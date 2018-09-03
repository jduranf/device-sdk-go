// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
//
// SPDX-License-Identifier: Apache-2.0
//
// This package provides a simple example implementation of
// a ProtocolDriver interface.
//
package modbus

import (
	"fmt"
	"strconv"
	"time"

	engine "github.com/edgexfoundry/device-sdk-go/examples/modbus/engine-modbus"

	//"github.com/edgexfoundry/edgex-go/core/domain/models"
	//logger "github.com/edgexfoundry/edgex-go/support/logging-client"
	//"github.com/tonyespy/gxds"

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
func (m *ModbusDriver) Initialize(lc logger.LoggingClient, asyncCh <-chan *device.CommandResult) error {
	m.lc = lc
	m.lc.Debug(fmt.Sprintf("ModbusHandler.Initialize called!"))

	return nil
}

// HandleOperation triggers an asynchronous protocol specific GET or SET operation
// for the specified device. Device profile attributes are passed as part
// of the *models.DeviceObject. The parameter 'value' must be provided for
// a SET operation, otherwise it should be 'nil'.
//
// This function is always called in a new goroutine. The driver is responsible
// for writing the CommandResults to the send channel.
//
// Note - DeviceObject represents a deviceResource defined in deviceprofile.
//
func (m *ModbusDriver) HandleOperation(ro *models.ResourceOperation,
	d *models.Device, do *models.DeviceObject, desc *models.ValueDescriptor,
	value string, send chan<- *device.CommandResult) {

	m.lc.Debug(fmt.Sprintf("HandleCommand: dev: %s op: %v attrs: %v", d.Name, ro.Operation, do.Attributes))
	var requestModbus engine.RequestModbus
	var requestResult engine.RequestResult
	//var engineModbus engine.EngineModbus
	var err error

	commconfig := d.Addressable.Address
	ch := make(chan engine.RequestResult, 10)

	err, engineModbus := engine.Create(commconfig, ch)
	if err != nil {
		m.lc.Debug(fmt.Sprintf("ModbusHandler.Error connecting Modbus"))
		return
	}

	//Fill requestModbus
	conv, _ := strconv.Atoi(do.Attributes["HoldingRegister"].(string))
	requestModbus.Address = uint16(conv)
	conv, _ = strconv.Atoi(do.Properties.Value.Size)
	requestModbus.NumRegs = uint16(conv)
	conv, _ = strconv.Atoi(d.Addressable.Path)
	requestModbus.SlaveId = byte(conv)
	requestModbus.Device = d.Name
	requestModbus.Description = do.Name
	//requestModbus.Timeout = engineModbus.
	typereg, _ := strconv.Atoi(do.Properties.Value.Type)

	requestModbus.Retry = 1

	times := time.Now().UnixNano() / int64(time.Millisecond)
	for index := 0; index < requestModbus.Retry; index++ {
		engineModbus.AddRequest(requestModbus)

		times = time.Now().UnixNano() / int64(time.Millisecond)
		requestResult = engineModbus.LaunchUnit()

		if requestResult.Err == nil {
			index = requestModbus.Retry
		}
	}

	cr := &device.CommandResult{RO: ro, DeviceId: string(d.Id), DeviceName: requestResult.Device, Origin: times, Type: device.ResultType(typereg), NumericResult: requestResult.Data}
	/*log.Println(requestResult)
	log.Println(cr)
	log.Println(cr.RO)
	log.Println(cr.DeviceId)
	log.Println(cr.DeviceName)
	log.Println(cr.NumericResult)
	*/
	send <- cr

}

// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (m *ModbusDriver) Stop(force bool) error {
	m.lc.Debug(fmt.Sprintf("Stop called: force=%v", force))
	return nil
}
