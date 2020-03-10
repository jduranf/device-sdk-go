// -*- mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"github.com/Circutor/edgex/pkg/clients/coredata"
	"github.com/Circutor/edgex/pkg/clients/logger"
	"github.com/Circutor/edgex/pkg/clients/metadata"
	"github.com/Circutor/edgex/pkg/models"
	ds_models "github.com/edgexfoundry/device-sdk-go/pkg/models"
)

var (
	ServiceName            string
	ServiceVersion         string
	CurrentConfig          *Config
	CurrentDeviceService   models.DeviceService
	ServiceLocked          bool
	Driver                 ds_models.ProtocolDriver
	EventClient            coredata.EventClient
	AddressableClient      metadata.AddressableClient
	DeviceClient           metadata.DeviceClient
	DeviceServiceClient    metadata.DeviceServiceClient
	DeviceProfileClient    metadata.DeviceProfileClient
	LoggingClient          logger.LoggingClient
	ProvisionWatcherClient metadata.ProvisionWatcherClient
)
