// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Circutor S.A.
//
// SPDX-License-Identifier: Apache-2.0

// This package provides a modbus example of a device service.
package main

import (
	"github.com/edgexfoundry/device-sdk-go/example/device-modbus"
	"github.com/edgexfoundry/device-sdk-go/pkg/startup"
)

const (
	version     string = "0.1"
	serviceName string = "device-modbus"
)

func main() {
	md := modbus.ModbusDriver{}
	startup.Bootstrap(serviceName, version, &md)
}
