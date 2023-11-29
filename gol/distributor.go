package gol

import (
	"fmt"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
)

//Broker address
var broker = "127.0.0.1:8040"

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
func tickerFunc(p Params, c distributorChannels, client *rpc.Client, done chan bool, request stubs.Request) {
	var turns int
	request2 := stubs.Request{
		Turns:       p.Turns,
		ImageHeight: p.ImageHeight,
		ImageWidth:  p.ImageWidth,
		Threads:     p.Threads,
	}

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
			response2 := new(stubs.Response)
			client.Call(stubs.Retrieve, request2, response2)

			if pause != true {
				c.events <- AliveCellsCount{
					CompletedTurns: response2.TurnsCompleted,
					CellsCount:     response2.AliveCount,
				}
				for i := 0; i < p.ImageHeight; i++ {
					for j := 0; j < p.ImageWidth; j++ {
						readWorld := response2.World[i][j]
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
				response2 := new(stubs.Response)
				client.Call(stubs.Retrieve, request, response2)
				turns = response2.TurnsCompleted
				for i := 0; i < p.ImageHeight; i++ {
					for j := 0; j < p.ImageWidth; j++ {
						writeWorld := response2.World[i][j]
						c.ioOutput <- writeWorld
					}
				}
				c.events <- StateChange{turns, Quitting}
				done <- true
				client.Call(stubs.Quit, request, response2)
			case 's':
				response2 := new(stubs.Response)
				client.Call(stubs.Retrieve, request, response2)
				var outFilename = fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
				println(outFilename)
				c.ioCommand <- ioOutput
				c.ioFilename <- outFilename
				for i := 0; i < p.ImageHeight; i++ {
					for j := 0; j < p.ImageWidth; j++ {
						writeWorld := response2.World[i][j]
						c.ioOutput <- writeWorld
					}
				}
			case 'k':
				response2 := new(stubs.Response)
				client.Call(stubs.Retrieve, request, response2)
				turns = response2.TurnsCompleted
				var outFilename = fmt.Sprintf("%vx%vx%v", p.ImageWidth, p.ImageHeight, p.Turns)
				c.ioCommand <- ioOutput
				c.ioFilename <- outFilename
				for i := 0; i < p.ImageHeight; i++ {
					for j := 0; j < p.ImageWidth; j++ {
						writeWorld := response2.World[i][j]
						c.ioOutput <- writeWorld
					}
				}
				c.events <- StateChange{turns, Quitting}
				done <- true
				client.Call(stubs.SuperQuit, request, response2)

			case 'p':
				responseP := new(stubs.Response)
				client.Call(stubs.Retrieve, request, responseP)
				if pause == false {
					turns = responseP.TurnsCompleted
					c.events <- StateChange{turns, Paused}
					client.Call(stubs.Pause, request, responseP)
					pause = true
				} else if pause == true {
					c.events <- StateChange{turns, Executing}
					client.Call(stubs.Pause, request, responseP)
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
	fmt.Println("Server: ", broker)
	client, _ := rpc.Dial("tcp", broker)
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

	go tickerFunc(p, c, client, done, request)
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
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- ImageOutputComplete{response.TurnsCompleted, outFilename}

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	done <- true
	close(c.events)

}
