package main

//QUICK NOTES
/*
- This is the Broker for reduced coupling between the Distributor and Workers
- Broker was created in tandem with the multiple threads
- Maximum amount of worker nodes is 8, but can be increased if necessary by the addition of addresses in the serverList
*/

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

//Channels for Shutdown
var waiting = make(chan bool)
var quitting = make(chan bool)
var superQuit = make(chan bool)

//Client connections stored in a list
var clients []rpc.Client

//Storage of the current World and turn for retrieval
//Set as global Variable so every function has access to it
var cTurn = 0
var cWorld [][]byte

//Mutex to avoid nasty overwrites
var mt sync.Mutex

//Check for connection
func isConnected(client *rpc.Client) bool {
	if client == nil {
		return false
	} else {
		return true
	}
}

func calculateAliveCells(p stubs.Parameters, world [][]byte) []util.Cell {
	var cs []util.Cell

	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			if world[i][j] != 0 {
				cs = append(cs, util.Cell{X: j, Y: i})
			}
		}
	}
	return cs
}

type Operations struct{}

func (s *Operations) Run(req stubs.Request, res *stubs.Response) (err error) {
	p := stubs.Parameters{Turns: req.Turns, Threads: req.Threads, ImageWidth: req.ImageWidth, ImageHeight: req.ImageHeight}

	world := req.World

	cWorld = make([][]byte, p.ImageWidth)
	for i := range world {
		cWorld[i] = make([]byte, p.ImageHeight)
	}

	res.World = make([][]byte, p.ImageWidth)
	for i := range world {
		res.World[i] = make([]byte, p.ImageHeight)
	}

	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			readWorld := req.World[i][j]
			res.World[i][j] = readWorld
		}
	}
	//
	//IF IT IS A SINGLE THREAD
	//
	if p.Threads == 1 {
	turnLoop:
		for turn := 0; turn < p.Turns; turn++ {
			//SELECT STATEMENT TO SEE IF WE WANT TO QUIT OR PAUSE THE TURNLOOP
			select {
			case <-quitting:
				break turnLoop

			case <-waiting:
				fmt.Println("State paused")
				<-waiting
				fmt.Println("Loop resumed")
			default:

				//WHEN THE TURN ITERATES THE SINGLE THREAD WILL ATTEMPT TO USE THE VALUES GIVEN TO IT
				workReq := stubs.Request{World: world, Turns: turn, ImageHeight: p.ImageHeight, ImageWidth: p.ImageWidth, StartY: 0, EndY: p.ImageHeight, Threads: p.Threads}
				workRes := new(stubs.Response)

				//WE CALL TO THE FIRST CLIENT AND MAKE AN RPC CALL FOR UPDATING THE WORLD BY 1 TURN
				clients[0].Call(stubs.GameOfLifeUpdate, workReq, workRes)
				mt.Lock()
				cTurn += 1
				res.TurnsCompleted = cTurn
				for i := 0; i < p.ImageHeight; i++ {
					for j := 0; j < p.ImageWidth; j++ {
						readWorld := workRes.WorkSlice[i][j]
						world[i][j] = readWorld
						cWorld[i][j] = readWorld
						res.World[i][j] = readWorld
					}
				}
				mt.Unlock()
			}
		}

		// AREA OF ERROR AREA OF ERROR
		//
		//
		//
		//
		//THIS IS WHEN THERE ARE MULTIPLE THREADS
		//MAKES WORKER CHANNELS FOR EACH INDIVIDUAL PROCESSES
	} else {

		//DEFINES A CHANNEL FOR EACH WORKER TO PASS THEIR WORK IN
		var works []chan [][]byte

		for i := 0; i < p.Threads; i++ {
			works = append(works, make(chan [][]byte))
		}
		println(works)

		////THIS IS WHERE THE TURNS WILL CONSTANTLY ITERATE UNTIL IT FINISHES
	turnloop2:
		for turn := 0; turn < p.Turns; turn++ {
			select {
			case <-quitting:
				break turnloop2

			case <-waiting:
				fmt.Println("State paused")
				<-waiting
				fmt.Println("Loop resumed")
			default:
				// create an empty world for appending the parts processed by the workers.
				world2 := make([][]byte, 0)

				///IF THE THREADS DIVIDES EVENLY
				if p.ImageHeight%p.Threads == 0 {
					for i := range works {

						StartY := i * p.ImageHeight / p.Threads
						EndY := (i + 1) * p.ImageHeight / p.Threads

						workerValue := i
						go func(worker int) {
							workReq := stubs.Request{World: world, Turns: turn, EndY: EndY, StartY: StartY, Worker: worker}
							workRes := new(stubs.Response)

							clients[worker].Call(stubs.GameOfLifeUpdate, workReq, workRes)

							works[worker] <- workRes.WorkSlice
						}(workerValue)
					}
					mt.Lock()
					for i := 0; i < p.Threads; i++ {
						world2 = append(world2, <-works[i]...)
					}
					mt.Unlock()

					world = world2

					mt.Lock()
					cTurn += 1
					res.TurnsCompleted = cTurn

					for i := 0; i < p.ImageHeight; i++ {
						for j := 0; j < p.ImageWidth; j++ {
							readWorld := world[i][j]
							cWorld[i][j] = readWorld
							res.World[i][j] = readWorld
						}
					}
					mt.Unlock()

					/// NON DIVISIBLE WORLD
				} else { // cannot be fully divided, e.g. p.Threads=3.
					for i := range works {
						if i != p.Threads-1 { // workers except last one
							StartY := i * p.ImageHeight / p.Threads
							EndY := (i + 1) * p.ImageHeight / p.Threads
							workerValue := i

							go func(worker int) {
								workReq := stubs.Request{World: world, Turns: turn, EndY: EndY, StartY: StartY, Worker: worker}
								workRes := new(stubs.Response)

								clients[worker].Call(stubs.GameOfLifeUpdate, workReq, workRes)

								works[worker] <- workRes.WorkSlice
							}(workerValue)

						} else { // last worker have to work more
							StartY := i * p.ImageHeight / p.Threads
							EndY := p.ImageHeight
							workerValue := i

							go func(worker int) {
								workReq := stubs.Request{World: world, Turns: turn, EndY: EndY, StartY: StartY, Worker: worker}
								workRes := new(stubs.Response)

								clients[worker].Call(stubs.GameOfLifeUpdate, workReq, workRes)

								works[worker] <- workRes.WorkSlice
							}(workerValue)
						}
					}

					for i := 0; i < p.Threads; i++ {
						world2 = append(world2, <-works[i]...)
					}

					world = world2
					mt.Lock()
					cTurn += 1
					res.TurnsCompleted = cTurn

					for i := 0; i < p.ImageHeight; i++ {
						for j := 0; j < p.ImageWidth; j++ {
							readWorld := world[i][j]
							cWorld[i][j] = readWorld
							res.World[i][j] = readWorld
						}
					}
					mt.Unlock()

				}
			}
		}
	}
	println(fmt.Sprintf("at the end of the program for %vx%vx%v :", p.ImageWidth, p.ImageHeight, p.Turns))
	println(world)

	res.Alive = calculateAliveCells(p, world)
	/// END OF MULTIPLE THREAD LOOP
	return
}

// AREA OF ERROR AREA OF ERROR
//
//
//
//
//THIS IS WHEN THERE ARE MULTIPLE THREADS
//MAKES WORKER CHANNELS FOR EACH INDIVIDUAL PROCESSES

func (s *Operations) Quit(req stubs.Request, res *stubs.Response) (err error) {
	quitting <- true
	return
}

func (s *Operations) SuperQuit(req stubs.Request, res *stubs.Response) (err error) {
	quitting <- true
	for i := range clients {
		println(fmt.Sprintf("quitting on worker %v", i))
		clients[i].Call(stubs.WorkerQuit, req, res)
	}
	superQuit <- true
	return
}

func (s *Operations) Pause(req stubs.Request, res *stubs.Response) (err error) {
	waiting <- true
	return
}

func (s *Operations) RetrieveCurrentData(req stubs.Request, res *stubs.Response) (err error) {
	p := stubs.Parameters{Turns: req.Turns, Threads: req.Threads, ImageWidth: req.ImageWidth, ImageHeight: req.ImageHeight}
	res.World = make([][]byte, p.ImageWidth)
	for i := 0; i < p.ImageHeight; i++ {
		res.World[i] = make([]byte, p.ImageHeight)
	}
	mt.Lock()
	retrievedTurn := cTurn
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			readWorld := cWorld[i][j]
			res.World[i][j] = readWorld
		}
	}
	mt.Unlock()

	res.Alive = calculateAliveCells(p, res.World)
	res.AliveCount = len(calculateAliveCells(p, res.World))
	res.TurnsCompleted = retrievedTurn

	return

}

//RUNS ON STARTUP, STARTS A LISTENER FOR DISTRIBUTOR AND CONNECTS TO 8 DIFFERENT WORKERS
func main() {
	pAddr := flag.String("port", "8040", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&Operations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	println(listener.Addr())

	serverAddresses := []string{
		//todo REPLACE EVERY ADDRESS HERE WITH AN AWS ADDRESS INSTEAD
		"127.0.0.1:8031",
		"127.0.0.1:8032",
		"127.0.0.1:8033",
		"127.0.0.1:8034",
		"127.0.0.1:8035",
		"127.0.0.1:8036",
		"127.0.0.1:8037",
		"127.0.0.1:8038",
		// Add more server addresses as needed
	}

	for i := 0; i < len(serverAddresses); i++ {
		client, _ := rpc.Dial("tcp", serverAddresses[i])
		if isConnected(client) {
			println("connected")
		} else {
			println("badcode")
		}
		clients = append(clients, *client)
	}

	go func() {
	serverLoop:
		for {
			select {
			case <-superQuit:
				println("Shutting down")
				listener.Close()
				break serverLoop

			}
		}
	}()
	rpc.Accept(listener)
	return
}
