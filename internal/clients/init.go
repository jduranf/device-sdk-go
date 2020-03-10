// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 IOTech Ltd
// Copyright (c) 2019 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Circutor/edgex/pkg/clients"
	"github.com/Circutor/edgex/pkg/clients/coredata"
	"github.com/Circutor/edgex/pkg/clients/logger"
	"github.com/Circutor/edgex/pkg/clients/metadata"
	"github.com/edgexfoundry/device-sdk-go/internal/common"
)

const clientCount int = 8

// InitDependencyClients triggers Service Client Initializer to establish connection to Metadata and Core Data Services
// through Metadata Client and Core Data Client.
// Service Client Initializer also needs to check the service status of Metadata and Core Data Services,
// because they are important dependencies of Device Service.
// The initialization process should be pending until Metadata Service and Core Data Service are both available.
func InitDependencyClients() error {
	if err := validateClientConfig(); err != nil {
		return err
	}

	initializeLoggingClient()

	if err := checkDependencyServices(); err != nil {
		return err
	}

	initializeClients()

	common.LoggingClient.Info("Service clients initialize successful.")
	return nil
}

func validateClientConfig() error {

	if len(common.CurrentConfig.Clients[common.ClientMetadata].Host) == 0 {
		return fmt.Errorf("fatal error; Host setting for Core Metadata client not configured")
	}

	if common.CurrentConfig.Clients[common.ClientMetadata].Port == 0 {
		return fmt.Errorf("fatal error; Port setting for Core Metadata client not configured")
	}

	if len(common.CurrentConfig.Clients[common.ClientData].Host) == 0 {
		return fmt.Errorf("fatal error; Host setting for Core Data client not configured")
	}

	if common.CurrentConfig.Clients[common.ClientData].Port == 0 {
		return fmt.Errorf("fatal error; Port setting for Core Ddata client not configured")
	}

	// TODO: validate other settings for sanity: maxcmdops, ...

	return nil
}

func initializeLoggingClient() {
	var logTarget string
	config := common.CurrentConfig

	if config.Logging.EnableRemote {
		logTarget = config.Clients[common.ClientLogging].Url() + clients.ApiLoggingRoute
		fmt.Println("EnableRemote is true, using remote logging service")
	} else {
		logTarget = config.Logging.File
		fmt.Println("EnableRemote is false, using local log file")
	}

	common.LoggingClient = logger.NewClient(common.ServiceName, config.Logging.EnableRemote, logTarget, config.Writable.LogLevel)
}

func checkDependencyServices() error {
	var dependencyList = []string{common.ClientData, common.ClientMetadata}

	var waitGroup sync.WaitGroup
	dependencyCount := len(dependencyList)
	waitGroup.Add(dependencyCount)
	checkingErrs := make(chan<- error, dependencyCount)

	for i := 0; i < dependencyCount; i++ {
		go func(wg *sync.WaitGroup, serviceName string) {
			defer wg.Done()
			if err := checkServiceAvailable(serviceName); err != nil {
				checkingErrs <- err
			}
		}(&waitGroup, dependencyList[i])
	}

	waitGroup.Wait()
	close(checkingErrs)

	if len(checkingErrs) > 0 {
		return fmt.Errorf("checking required dependencied services failed ")
	} else {
		return nil
	}
}

func checkServiceAvailable(serviceId string) error {
	for i := 0; i < common.CurrentConfig.Service.ConnectRetries; i++ {
		if checkServiceAvailableByPing(serviceId) == nil {
			return nil
		}
		time.Sleep(time.Duration(common.CurrentConfig.Service.Timeout) * time.Millisecond)
		common.LoggingClient.Debug(fmt.Sprintf("Checked %d times for %s availibility", i+1, serviceId))
	}

	errMsg := fmt.Sprintf("service dependency %s checking time out", serviceId)
	common.LoggingClient.Error(errMsg)
	return fmt.Errorf(errMsg)
}

func checkServiceAvailableByPing(serviceId string) error {
	common.LoggingClient.Info(fmt.Sprintf("Check %v service's status ...", serviceId))
	addr := common.CurrentConfig.Clients[serviceId].Url()
	timeout := int64(common.CurrentConfig.Clients[serviceId].Timeout) * int64(time.Millisecond)

	client := http.Client{
		Timeout: time.Duration(timeout),
	}

	_, err := client.Get(addr + clients.ApiPingRoute)

	if err != nil {
		common.LoggingClient.Error(fmt.Sprintf("Error getting ping: %v ", err))
	}
	return err
}

func initializeClients() {
	// Initialize Core Metadata clients
	metaAddr := common.CurrentConfig.Clients[common.ClientMetadata].Url()

	url := metaAddr + clients.ApiAddressableRoute
	common.AddressableClient = metadata.NewAddressableClient(url)

	url = metaAddr + clients.ApiDeviceRoute
	common.DeviceClient = metadata.NewDeviceClient(url)

	url = metaAddr + clients.ApiDeviceServiceRoute
	common.DeviceServiceClient = metadata.NewDeviceServiceClient(url)

	url = metaAddr + clients.ApiDeviceProfileRoute
	common.DeviceProfileClient = metadata.NewDeviceProfileClient(url)

	url = metaAddr + clients.ApiProvisionWatcherRoute
	common.ProvisionWatcherClient = metadata.NewProvisionWatcherClient(url)

	// Initialize Core Data clients
	dataAddr := common.CurrentConfig.Clients[common.ClientData].Url()

	url = dataAddr + clients.ApiEventRoute
	common.EventClient = coredata.NewEventClient(url)
}
