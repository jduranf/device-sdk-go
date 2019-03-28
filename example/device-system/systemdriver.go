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
	"github.com/edgexfoundry/go-mod-core-contracts/clients/logging"
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

const gpioStatusO1 = "/sys/class/gpio/gpio9/value"
const gpioStatusO2 = "/sys/class/gpio/gpio136/value"
const cpuTemp = "/sys/class/thermal/thermal_zone0/temp"

var statsValues Stats

// DisconnectDevice handles protocol-specific cleanup when a device
// is removed.
func (sys *SystemDriver) DisconnectDevice(deviceName string, protocols map[string]map[string]string) error {
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
func (sys *SystemDriver) HandleReadCommands(deviceName string, protocols map[string]map[string]string, reqs []ds_models.CommandRequest) (res []*ds_models.CommandValue, err error) {

	res = make([]*ds_models.CommandValue, len(reqs))
	for i := range reqs {
		sys.lc.Debug(fmt.Sprintf("SystemDriver.HandleReadCommands: protocols: %v op: %v attrs: %v", protocols, reqs[i].RO.Operation, reqs[i].DeviceResource.Attributes))

		var value uint64
		var tmp int32
		if reqs[i].DeviceResource.Name != "CpuTemp" {
			value, err = getValue(reqs[i].DeviceResource.Name)
			if err != nil {
				sys.lc.Warn(fmt.Sprintf("Error getting system data: %v", err))
				return
			}
		} else {
			tmp, err = getTemp()
			if err != nil {
				sys.lc.Warn(fmt.Sprintf("Error getting cpu temperature: %v", err))
				return
			}
		}

		now := time.Now().UnixNano() / int64(time.Millisecond)
		if reqs[i].DeviceResource.Name == "StatusO1" || reqs[i].DeviceResource.Name == "StatusO2" {
			status := false
			if value == 1 {
				status = true
			}
			cv, _ := ds_models.NewBoolValue(&reqs[i].RO, now, status)
			res[i] = cv
		} else {
			if reqs[i].DeviceResource.Name == "CpuTemp" {
				cv, _ := ds_models.NewInt32Value(&reqs[i].RO, now, tmp)
				res[i] = cv
			} else {
				cv, _ := ds_models.NewUint64Value(&reqs[i].RO, now, value)
				res[i] = cv
			}

		}
	}
	return
}

// HandleWriteCommands passes a slice of CommandRequest struct each representing
// a ResourceOperation for a specific device resource (aka DeviceObject).
// Since the commands are actuation commands, params provide parameters for the individual
// command.
func (sys *SystemDriver) HandleWriteCommands(deviceName string, protocols map[string]map[string]string, reqs []ds_models.CommandRequest,
	params []*ds_models.CommandValue) error {

	if len(reqs) != 1 {
		err := fmt.Errorf("SystemDriver.HandleWriteCommands; too many command requests; only one supported")
		return err
	}
	if len(params) != 1 {
		err := fmt.Errorf("SystemDriver.HandleWriteCommands; the number of parameter is not correct; only one supported")
		return err
	}

	if reqs[0].DeviceResource.Name == "StatusO1" {
		if params[0].NumericValue[0] == 0 {
			ioutil.WriteFile(gpioStatusO1, []byte("0"), 0644)
		} else {
			ioutil.WriteFile(gpioStatusO1, []byte("1"), 0644)
		}
	}

	if reqs[0].DeviceResource.Name == "StatusO2" {
		if params[0].NumericValue[0] == 0 {
			ioutil.WriteFile(gpioStatusO2, []byte("0"), 0644)
		} else {
			ioutil.WriteFile(gpioStatusO2, []byte("1"), 0644)
		}
	}

	if reqs[0].DeviceResource.Name == "Reboot" {
		if params[0].NumericValue[0] != 0 {
			go waitToReboot(sys)
		}
	}

	sys.lc.Debug(fmt.Sprintf("SystemDriver.HandleWriteCommands: protocols: %v, operation: %v, parameters: %v", protocols, reqs[0].RO.Operation, params))
	return nil
}

// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (sys *SystemDriver) Stop(force bool) error {
	sys.lc.Debug(fmt.Sprintf("SystemDriver.Stop called: force=%v", force))
	return nil
}

// Discover triggers protocol specific device discovery, which is
// a synchronous operation which returns a list of new devices
// which may be added to the device service based on service
// config. This function may also optionally trigger sensor
// discovery, which could result in dynamic device profile creation.
func (sys *SystemDriver) Discover() error {
	sys.lc.Debug(fmt.Sprintf("SystemDriver.Discover called"))
	err := fmt.Errorf("SystemDriver.Discover unimplemented")
	return err
}

func getValue(request string) (value uint64, err error) {
	if request == "RamUsage" {
		info := syscall.Sysinfo_t{}
		err = syscall.Sysinfo(&info)
		if err != nil {
			err = fmt.Errorf("Error getting RAM usage: %v", err)
			return
		}

		totalRAM := uint64(info.Totalram)
		freeRAM := uint64(info.Freeram)
		value = ((totalRAM - freeRAM) * 100) / totalRAM
	} else if request == "DiskUsage" {
		var stat syscall.Statfs_t
		err = syscall.Statfs("/", &stat)
		if err != nil {
			return
		}

		free := uint64(stat.Bfree) * uint64(stat.Bsize)
		total := uint64(stat.Blocks) * uint64(stat.Bsize)
		used := total - free
		value = (used * 100) / total
	} else if request == "Uptime" {
		value = uint64(getUptime())
	} else if request == "StatusO1" {
		inputStr, _ := readFile(gpioStatusO1)
		if len(inputStr) != 0 {
			input, _ := strconv.Atoi(inputStr[0:1])
			value = uint64(input)
		}
	} else if request == "StatusO2" {
		inputStr, _ := readFile(gpioStatusO2)
		if len(inputStr) != 0 {
			input, _ := strconv.Atoi(inputStr[0:1])
			value = uint64(input)
		}
	} else if request == "EthRx" {
		value = statsValues.usageRx
	} else if request == "EthTx" {
		value = statsValues.usageTx
	} else if request == "CpuUsage" {
		value = statsValues.cpuUsage
	}
	return
}

func getTemp() (int32, error) {
	var value int32
	inputStr, err := readFile(cpuTemp)
	if err != nil {
		return value, err
	}
	if len(inputStr) != 0 {
		var input int
		input, err = strconv.Atoi(inputStr[0 : len(inputStr)-1])
		value = int32(input / 1000)
	}
	return value, err
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

			total := uint64(currentTotal - statsValues.cpuTotal)
			idled := uint64(currentIdle - statsValues.cpuIdle)

			statsValues.cpuIdle = currentIdle
			statsValues.cpuTotal = currentTotal
			statsValues.cpuUsage = uint64(((total - idled) * 100) / total)
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
				deltaTime := int(currentUptime - statsValues.uptime)
				statsValues.usageRx = uint64((rxBytes - statsValues.rxBytes) / deltaTime)
				statsValues.usageTx = uint64((txBytes - statsValues.txBytes) / deltaTime)
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

func waitToReboot(sys *SystemDriver) {
	sys.lc.Info(fmt.Sprintf("Executing Reboot System"))
	time.Sleep(3 * time.Second)
	_, err := exec.Command("reboot").Output()
	if err != nil {
		sys.lc.Info(fmt.Sprintf("Error Executing Reboot System"))
	}
}
