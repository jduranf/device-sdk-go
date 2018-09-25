// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Circutor S.A.
//
// SPDX-License-Identifier: Apache-2.0
//
// This package provides a Modbus device service.
//
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	device "github.com/edgexfoundry/device-sdk-go"
	"github.com/edgexfoundry/device-sdk-go/examples/modbus"
)

const (
	serviceName    = "device-modbus"
	serviceVersion = "0.1"
)

var flags struct {
	configPath *string
}

func main() {
	var useRegistry bool
	var profile string
	var confDir string

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flag.BoolVar(&useRegistry, "registry", false, "Indicates the service should use the registry.")
	flag.BoolVar(&useRegistry, "r", false, "Indicates the service should use registry.")
	flag.StringVar(&profile, "profile", "", "Specify a profile other than default.")
	flag.StringVar(&profile, "p", "", "Specify a profile other than default.")
	flag.StringVar(&confDir, "confdir", "", "Specify an alternate configuration directory.")
	flag.StringVar(&confDir, "c", "", "Specify an alternate configuration directory.")
	flag.Parse()

	if err := startService(useRegistry, profile, confDir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func startService(useRegistry bool, profile string, confDir string) error {
	md := modbus.ModbusDriver{}

	s, err := device.NewService(serviceName, serviceVersion, &md)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Calling service.Start.\n")

	if err := s.Start(useRegistry, profile, confDir); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Setting up signals.\n")

	// TODO: this code never executes!

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-ch:
		fmt.Fprintf(os.Stderr, "Exiting on %s signal.\n", sig)
	}

	//return s.Stop(false)
	return s.Stop(true)
}
