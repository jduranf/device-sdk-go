package engine

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goburrow/modbus"
)

// Modbus request
type RequestModbus struct {
	id              int
	SlaveId         byte
	BaudRate        int
	Operation       int
	Address         uint16
	NumRegs         uint16
	Frequency       time.Duration
	Retry           int
	Timeout         time.Duration
	TimeQuietBefore int
	Priority        int
	Quarentine      int
	nextLaunch      time.Time
	Device          string
	Description     string
}

// Request result
type RequestResult struct {
	Id          int
	Device      string
	Description string
	Data        []byte
	Err         error
}

// Engine Modbus variables
type EngineModbus struct {
	id            int
	mutex         sync.Mutex
	handler       *modbus.RTUClientHandler
	requestModbus RequestModbus
	client        modbus.Client
	ch            chan RequestResult
}

// Starts a Modbus RTU engine.
func Create(commconfig string, ch chan RequestResult) (error, *EngineModbus) {
	var conf []string

	engine := new(EngineModbus)

	engine.id = 1
	engine.ch = ch

	commconfig = strings.Replace(commconfig, ",", " ", -1)
	conf = strings.Fields(commconfig)

	//TODO: if fields are no correct, create engine default can not connect correctly

	// Modbus RTU/ASCII
	engine.handler = modbus.NewRTUClientHandler(conf[0])
	engine.handler.BaudRate, _ = strconv.Atoi(conf[1])
	engine.handler.DataBits, _ = strconv.Atoi(conf[2])
	engine.handler.StopBits, _ = strconv.Atoi(conf[3])
	engine.handler.Parity = conf[4]
	engine.handler.SlaveId = 1
	engine.handler.Timeout = 2000 * time.Millisecond

	// Connect
	err := engine.handler.Connect()
	if err == nil {
		engine.client = modbus.NewClient(engine.handler)
	}

	return err, engine
}

// Add request
func (engine *EngineModbus) AddRequest(request RequestModbus) int {
	// Mutex to avoid conflicts
	engine.mutex.Lock()
	defer engine.mutex.Unlock()

	// Set request Id
	request.id = engine.id
	//engine.id++

	// Set next launch time
	//request.nextLaunch = time.Now().Add(request.Frequency)

	// Add request
	engine.requestModbus = request

	return request.id
}

/*
// Remove request
func (engine *EngineModbus) RemoveRequest(id int) bool {
	// Mutex to avoid conflicts
	engine.mutex.Lock()
	defer engine.mutex.Unlock()

	// Search request with the same ID
	for i := 0; i < len(engine.requestModbus); i++ {
		if engine.requestModbus[i].id == id {
			// Remove request
			engine.requestModbus = append(engine.requestModbus[:i], engine.requestModbus[i+1:]...)
			log.Println("Request Id", id, "removed")
			return true
		}
	}

	return false
}
*/

// Launch engine, should be done using a goroutine
/*func (engine *EngineModbus) Launch() {
	var busy bool
	var result RequestResult
	for {
		busy = false
		t := time.Now()
		engine.mutex.Lock()
		for i := 0; i < len(engine.requestModbus); i++ {
			// Check if time elapsed
			elapsed := t.Sub(engine.requestModbus[i].nextLaunch)
			if elapsed >= 0 {
				// Perform request
				engine.handler.SlaveId = engine.requestModbus[i].SlaveId
				result.Id = engine.requestModbus[i].id

				result.Data, result.Err = engine.client.ReadInputRegisters(engine.requestModbus[i].Address, engine.requestModbus[i].NumRegs)

				result.Device = engine.requestModbus[i].Device
				result.Description = engine.requestModbus[i].Description

				// Send result
				engine.ch <- result

				// Calculate next launch time
				engine.requestModbus[i].nextLaunch = engine.requestModbus[i].nextLaunch.Add(engine.requestModbus[i].Frequency)
				elapsed = time.Now().Sub(engine.requestModbus[i].nextLaunch)
				if elapsed >= 0 {
					engine.requestModbus[i].nextLaunch = time.Now().Add(engine.requestModbus[i].Frequency)
				}
				busy = true
			}
		}
		engine.mutex.Unlock()
		for i := 0; i < len(engine.requestModbus); i++ {
			if engine.requestModbus[i].Frequency == 0 {
				engine.RemoveRequest(engine.requestModbus[i].id)
			}
		}

		if busy == false {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Launch engine, should be done using a goroutine
func (engine *EngineModbus) LaunchSimple() {
	var busy bool
	var result RequestResult
	for {
		busy = false
		engine.mutex.Lock()
		for i := 0; i < len(engine.requestModbus); i++ {
			// Perform request
			engine.handler.SlaveId = engine.requestModbus[i].SlaveId
			result.Id = engine.requestModbus[i].id

			result.Data, result.Err = engine.client.ReadInputRegisters(engine.requestModbus[i].Address, engine.requestModbus[i].NumRegs)

			result.Device = engine.requestModbus[i].Device
			result.Description = engine.requestModbus[i].Description

			// Send result
			engine.ch <- result

			busy = true
		}
	}
	engine.mutex.Unlock()
	engine.requestModbus = nil

	if busy == false {
		time.Sleep(100 * time.Millisecond)
	}
}
*/
// Launch engine, should be done using a goroutine
func (engine *EngineModbus) LaunchUnit() RequestResult {
	var result RequestResult

	engine.mutex.Lock()

	// Perform request
	engine.handler.SlaveId = engine.requestModbus.SlaveId
	result.Id = engine.requestModbus.id

	result.Data, result.Err = engine.client.ReadInputRegisters(engine.requestModbus.Address, engine.requestModbus.NumRegs)

	result.Device = engine.requestModbus.Device
	result.Description = engine.requestModbus.Description

	// Send result
	//engine.ch <- result

	engine.mutex.Unlock()
	//engine.requestModbus = nil
	return result

}
