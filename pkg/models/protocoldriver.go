// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018-2019 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// This package defines an interface used to build an EdgeX Foundry device
// service.  This interace provides an asbstraction layer for the device
// or protocol specific logic of a device service.
//
package models

import (
	"github.com/Circutor/edgex/pkg/clients/logger"
	"github.com/Circutor/edgex/pkg/models"
)

// ProtocolDriver is a low-level device-specific interface used by
// by other components of an EdgeX device service to interact with
// a specific class of devices.
type ProtocolDriver interface {

	// DisconnectDevice is when a device is removed from the device
	// service. This function allows for protocol specific disconnection
	// logic to be performed.  Device services which don't require this
	// function should just return 'nil'.
	//
	DisconnectDevice(deviceName string, protocols map[string]models.ProtocolProperties) error

	// Initialize performs protocol-specific initialization for the device
	// service. The given *AsyncValues channel can be used to push asynchronous
	// events and readings to Core Data.
	Initialize(lc logger.LoggingClient, asyncCh chan<- *AsyncValues) error

	// HandleReadCommands passes a slice of CommandRequest struct each representing
	// a ResourceOperation for a specific device resource.
	HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []CommandRequest) ([]*CommandValue, error)

	// HandleWriteCommands passes a slice of CommandRequest struct each representing
	// a ResourceOperation for a specific device resource.
	// Since the commands are actuation commands, params provide parameters for the individual
	// command.
	HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []CommandRequest, params []*CommandValue) error

	// Stop instructs the protocol-specific DS code to shutdown gracefully, or
	// if the force parameter is 'true', immediately. The driver is responsible
	// for closing any in-use channels, including the channel used to send async
	// readings (if supported).
	Stop(force bool) error

	// Discover triggers protocol specific device discovery, which is
	// a synchronous operation which returns a list of new devices
	// which may be added to the device service based on service
	// config. This function may also optionally trigger sensor
	// discovery, which could result in dynamic device profile creation.
	Discover() (err error)
}
