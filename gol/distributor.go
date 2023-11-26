package gol

import (
	"flag"
	"fmt"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
)

//set as a global variable as it is only being run once
var server = flag.String("server", "127.0.0.1:8040", "IP:port string to connect to as server")

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keypresses <-chan rune
}

//This Ticker function is constantly running when a GOL state is being calculated, will only stop once all the turns have finished
func tickerFunc(p Params, c distributorChannels, client *rpc.Client, done chan bool, request stubs.Request, response *stubs.Response) {

	var currentWorld = make([][]byte, p.ImageWidth)
	for i := range currentWorld {
		currentWorld[i] = make([]byte, p.ImageHeight)
	}

	var pause = false
	ticker := time.NewTicker(2 * time.Second)
loop:
	for {
		select {
		case <-ticker.C:
			if pause != true {
				client.Call(stubs.Retrieve, request, response)
				c.events <- AliveCellsCount{
					CompletedTurns: response.TurnsCompleted,
					CellsCount:     response.AliveCount,
				}
				for i := 0; i < p.ImageHeight; i++ {
					for j := 0; j < p.ImageWidth; j++ {
						readWorld := response.World[i][j]
						currentWorld[i][j] = readWorld

					}
				}
			}

		case k := <-c.keypresses:
			switch k {
			case 'q':
				var outFilename = fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
				c.ioCommand <- ioOutput
				c.ioFilename <- outFilename

				for i := 0; i < p.ImageHeight; i++ {
					for j := 0; j < p.ImageWidth; j++ {
						writeWorld := currentWorld[i][j]
						c.ioOutput <- writeWorld
					}
				}
				c.events <- StateChange{response.TurnsCompleted, Quitting}
				done <- true
				client.Call(stubs.Quit, request, response)
			case 's':
				var outFilename = fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
				println(outFilename)
				c.ioCommand <- ioOutput
				c.ioFilename <- outFilename

				for i := 0; i < p.ImageHeight; i++ {
					for j := 0; j < p.ImageWidth; j++ {
						writeWorld := response.World[i][j]
						c.ioOutput <- writeWorld
					}
				}

			case 'k':
				var outFilename = fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
				c.ioCommand <- ioOutput
				c.ioFilename <- outFilename

				for i := 0; i < p.ImageHeight; i++ {
					for j := 0; j < p.ImageWidth; j++ {
						writeWorld := currentWorld[i][j]
						c.ioOutput <- writeWorld
					}
				}
				c.events <- StateChange{response.TurnsCompleted, Quitting}
				done <- true
				client.Call(stubs.SuperQuit, request, response)

			case 'p':
				if pause == false {
					c.events <- StateChange{response.TurnsCompleted, Paused}
					client.Call(stubs.Pause, request, response)
					pause = true
				} else if pause == true {
					c.events <- StateChange{response.TurnsCompleted, Executing}
					client.Call(stubs.Pause, request, response)
					pause = false
				}
			}

		case <-done:
			break loop

		}
	}
}

func distributor(p Params, c distributorChannels) {
	done := make(chan bool, 1)
	turn := 0
	flag.Parse()
	fmt.Println("Server: ", *server)
	client, _ := rpc.Dial("tcp", *server)
	defer client.Close()

	world := make([][]byte, p.ImageWidth)
	for i := range world {
		world[i] = make([]byte, p.ImageHeight)
	}

	var filename = fmt.Sprintf("%vx%v", p.ImageWidth, p.ImageHeight)
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			readWorld := <-c.ioInput
			world[i][j] = readWorld
		}
	}

	request := stubs.Request{World: world, Turns: p.Turns, ImageHeight: p.ImageHeight, ImageWidth: p.ImageWidth, Threads: p.Threads}
	response := new(stubs.Response)

	go tickerFunc(p, c, client, done, request, response)
	client.Call(stubs.BrokeOps, request, response)

	turn = response.TurnsCompleted

	c.events <- FinalTurnComplete{turn, response.Alive}

	var outFilename = fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
	c.ioCommand <- ioOutput
	c.ioFilename <- outFilename
	for i := 0; i < p.ImageHeight; i++ {
		for j := 0; j < p.ImageWidth; j++ {
			writeWorld := response.World[i][j]
			c.ioOutput <- writeWorld
		}
	}
	c.events <- ImageOutputComplete{response.TurnsCompleted, outFilename}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	done <- true
	close(c.events)
	println("events closed")

}
