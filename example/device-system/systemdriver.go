// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Circutor S.A.
//
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/edgexfoundry/device-sdk-go/example/device-system/comp"
	ds_models "github.com/edgexfoundry/device-sdk-go/pkg/models"
	"github.com/edgexfoundry/edgex-go/pkg/clients/logging"
	"github.com/edgexfoundry/edgex-go/pkg/models"
)

type SystemDriver struct {
	lc      logger.LoggingClient
	asyncCh chan<- *ds_models.AsyncValues
}

type Stats struct {
	cpuIdle  int
	cpuTotal int
	cpuUsage uint64

	rxBytes int
	txBytes int
	usageRx uint64
	usageTx uint64
	uptime  int64
}

var statsValues Stats

// DisconnectDevice handles protocol-specific cleanup when a device
// is removed.
func (sys *SystemDriver) DisconnectDevice(address *models.Addressable) error {
	return nil
}

// Initialize performs protocol-specific initialization for the device
// service.
func (sys *SystemDriver) Initialize(lc logger.LoggingClient, asyncCh chan<- *ds_models.AsyncValues) error {
	sys.lc = lc
	sys.asyncCh = asyncCh
	go refreshStats()
	return nil
}

// HandleReadCommands triggers a protocol Read operation for the specified device.
func (sys *SystemDriver) HandleReadCommands(addr *models.Addressable, reqs []ds_models.CommandRequest) (res []*ds_models.CommandValue, err error) {

	res = make([]*ds_models.CommandValue, len(reqs))
	for i := range reqs {
		sys.lc.Debug(fmt.Sprintf("SystemDriver.HandleReadCommands: dev: %s op: %v attrs: %v", addr.Name, reqs[i].RO.Operation, reqs[i].DeviceObject.Attributes))

		var value uint64
		value, err = getValue(reqs[i].DeviceObject.Name)
		if err != nil {
			sys.lc.Warn(fmt.Sprintf("Error getting system data: %v", err))
			return
		}

		now := time.Now().UnixNano() / int64(time.Millisecond)
		if reqs[i].DeviceObject.Name == "STATUS_O1" || reqs[i].DeviceObject.Name == "STATUS_O2" {
			status := false
			if value == 1 {
				status = true
			}
			cv, _ := ds_models.NewBoolValue(&reqs[i].RO, now, status)
			res[i] = cv
		} else {
			cv, _ := ds_models.NewUint64Value(&reqs[i].RO, now, value)
			res[i] = cv
		}
	}
	return
}

// HandleWriteCommands passes a slice of CommandRequest struct each representing
// a ResourceOperation for a specific device resource (aka DeviceObject).
// Since the commands are actuation commands, params provide parameters for the individual
// command.
func (sys *SystemDriver) HandleWriteCommands(addr *models.Addressable, reqs []ds_models.CommandRequest,
	params []*ds_models.CommandValue) error {

	err := fmt.Errorf("SystemDriver.HandleWriteCommands not implemented")
	return err
}

// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (sys *SystemDriver) Stop(force bool) error {
	sys.lc.Debug(fmt.Sprintf("SystemDriver.Stop called: force=%v", force))
	return nil
}

func getValue(request string) (value uint64, err error) {
	if request == "RAM_USAGE" {
		info := syscall.Sysinfo_t{}
		err = syscall.Sysinfo(&info)
		if err != nil {
			err = fmt.Errorf("Error getting RAM usage: %v", err)
			return
		}

		value = (uint64)(((info.Totalram - info.Freeram) * 100) / info.Totalram)
	} else if request == "DISK_USAGE" {
		var stat syscall.Statfs_t
		err = syscall.Statfs("/", &stat)
		if err != nil {
			return
		}

		free := stat.Bfree * uint64(stat.Bsize)
		total := stat.Blocks * uint64(stat.Bsize)
		used := total - free
		value = (used * 100) / total
	} else if request == "UPTIME" {
		value = (uint64)(getUptime())
	} else if request == "STATUS_O1" {
		var input string
		input, _ = readFile("/sys/class/gpio/gpio9/value")
		if input == "1" {
			value = 0
		}
	} else if request == "STATUS_O2" {
		var input string
		input, _ = readFile("/sys/class/gpio/gpio136/value")
		if input == "1" {
			value = 0
		}
	} else if request == "REBOOT" {
		_, err = exec.Command("reboot").Output()
		if err != nil {
			err = fmt.Errorf("Error executing reboot: %v", err)
			return
		}
		value = 1
	} else if request == "ETH_RX" {
		value = statsValues.usageRx
	} else if request == "ETH_TX" {
		value = statsValues.usageTx
	} else if request == "CPU_USAGE" {
		value = statsValues.cpuUsage
	}
	return
}

func stringBetween(value string, a string, b string) string {
	// Get substring between two strings.
	posFirst := strings.Index(value, a)
	if posFirst == -1 {
		return ""
	}
	posLast := strings.Index(value, b)
	if posLast == -1 {
		return ""
	}
	posFirstAdjusted := posFirst + len(a)
	if posFirstAdjusted >= posLast {
		return ""
	}
	return value[posFirstAdjusted:posLast]
}

func getUptime() int64 {
	info := syscall.Sysinfo_t{}
	syscall.Sysinfo(&info)
	return (int64)(info.Uptime)
}

func refreshStats() {

	for {
		// Get wifi status and refresh LED
		technologies, err := exec.Command("connmanctl", "technologies").Output()
		if err == nil {
			wifi := stringBetween(string(technologies), "Type = wifi", "/net/connman/technology/ethernet")
			if strings.Contains(wifi, "Connected = True") {
				ioutil.WriteFile("/sys/class/leds/wlan_blue_led/brightness", []byte("1"), 0644)
			} else {
				ioutil.WriteFile("/sys/class/leds/wlan_blue_led/brightness", []byte("0"), 0644)
			}
		}

		// Refresh CPU usage
		procstat, err := readFile("/proc/stat")
		if err == nil {
			//https: //stackoverflow.com/questions/23367857/accurate-calculation-of-cpu-usage-given-in-percentage-in-linux
			procstat = stringBetween(string(procstat), "cpu  ", "cpu0")
			transf := strings.Fields(procstat)
			user, _ := strconv.Atoi(transf[0])
			nice, _ := strconv.Atoi(transf[1])
			system, _ := strconv.Atoi(transf[2])
			idle, _ := strconv.Atoi(transf[3])
			iowait, _ := strconv.Atoi(transf[4])
			irq, _ := strconv.Atoi(transf[5])
			softirq, _ := strconv.Atoi(transf[6])
			steal, _ := strconv.Atoi(transf[7])

			currentIdle := idle + iowait
			currentNoIdle := user + nice + system + irq + softirq + steal
			currentTotal := currentIdle + currentNoIdle

			total := currentTotal - statsValues.cpuTotal
			idled := currentIdle - statsValues.cpuIdle

			statsValues.cpuIdle = currentIdle
			statsValues.cpuTotal = currentTotal
			statsValues.cpuUsage = (uint64)(((total - idled) * 100) / total)
		}

		// Refresh ethernet usage
		ethrx, err := readFile(comp.EthRXUsageFile)
		if err == nil {
			ethtx, err := readFile(comp.EthTXUsageFile)
			if err == nil {
				aux := string(ethtx)
				transf := strings.Fields(aux)
				txBytes, _ := strconv.Atoi(transf[0])

				aux = string(ethrx)
				transf = strings.Fields(aux)
				rxBytes, _ := strconv.Atoi(transf[0])

				currentUptime := getUptime()
				deltaTime := (int)(currentUptime - statsValues.uptime)
				statsValues.usageRx = (uint64)((rxBytes - statsValues.rxBytes) / deltaTime)
				statsValues.usageTx = (uint64)((txBytes - statsValues.txBytes) / deltaTime)
				statsValues.rxBytes = rxBytes
				statsValues.txBytes = txBytes
				statsValues.uptime = currentUptime
			}
		}

		// Wait before refresh values
		time.Sleep(15 * time.Second)
	}
}

func readFile(file string) (dat string, e error) {
	d, e := ioutil.ReadFile(file)
	dat = string(d[:])
	return dat, e
}
