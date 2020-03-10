// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"fmt"
	"sync"

	"github.com/Circutor/edgex/pkg/models"
	"github.com/edgexfoundry/device-sdk-go/internal/common"
	"github.com/google/uuid"
)

var (
	initOnce sync.Once
)

// Init basic state for cache
func InitCache() {
	initOnce.Do(func() {
		ctx := context.WithValue(context.Background(), common.CorrelationHeader, uuid.New().String())

		ds, err := common.DeviceClient.DevicesForServiceByName(common.ServiceName, ctx)
		if err != nil {
			common.LoggingClient.Error(fmt.Sprintf("Device cache initialization failed: %v", err))
			ds = make([]models.Device, 0)
		}
		newDeviceCache(ds)

		pws, err := common.ProvisionWatcherClient.ProvisionWatchersForServiceByName(common.ServiceName, ctx)
		if err != nil {
			common.LoggingClient.Error(fmt.Sprintf("Provision Watchers cache initialization failed: %v", err))
			pws = make([]models.ProvisionWatcher, 0)
		}
		newWatcherCache(pws)

		dps := []models.DeviceProfile{}
		for _, d := range ds {
			dps = append(dps, d.Profile)
		}
		for _, pw := range pws {
			dps = append(dps, pw.Profile)
		}
		newProfileCache(dps)
	})
}
