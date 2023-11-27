package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
)

var workerQuit = make(chan bool)
var iteration = 1

//the bread and butter of the worker, calculates the next state of the Game Of Life
func calculateNextState(startY, endY int, world [][]byte) [][]byte {
	// create a new state
	var newHeight = endY - startY
	newWorld := make([][]byte, newHeight)
	for j := range newWorld {
		newWorld[j] = make([]byte, len(world[j]))
	}

	// rules of the game
	for y := 0; y < newHeight; y++ {
		for x := 0; x < len(world); x++ {
			if world[y+startY][x] == 0 {
				if calculateSurroundings(y+startY, x, world) == 3 {
					newWorld[y][x] = 255
				}
			}
			if world[y+startY][x] == 255 {
				if calculateSurroundings(y+startY, x, world) < 2 || calculateSurroundings(y+startY, x, world) > 3 {
					newWorld[y][x] = 0
				}
				if calculateSurroundings(y+startY, x, world) == 2 || calculateSurroundings(y+startY, x, world) == 3 {
					newWorld[y][x] = world[y+startY][x]
				}
			}
		}
	}
	return newWorld
}

func calculateSurroundings(row, column int, world [][]byte) int {
	count := 0
	rowAbove := row - 1
	rowBelow := row + 1
	if row == 0 {
		rowAbove = len(world[0]) - 1
	} else if row == len(world[0])-1 {
		rowBelow = 0
	}
	columnLeft := column - 1
	columnRight := column + 1
	if column == 0 {
		columnLeft = len(world[0]) - 1
	} else if column == len(world[0])-1 {
		columnRight = 0
	}
	// make a list full of neighbours
	surroundings := []byte{world[rowAbove][columnLeft], world[rowAbove][column], world[rowAbove][columnRight],
		world[row][columnLeft], world[row][columnRight], world[rowBelow][columnLeft], world[rowBelow][column],
		world[rowBelow][columnRight]}
	for _, surrounding := range surroundings {
		if surrounding == 255 {
			count = count + 1
		}
	}
	return count
}

// GameOfLifeOperations This is the method that is going to be RPC Called
type GameOfLifeOperations struct {
}

// Update THIS IS THE UPDATE FUNCTION, THE BREAD AND BUTTER OF THE SYSTEM
func (s *GameOfLifeOperations) Update(req stubs.Request, res *stubs.Response) (err error) {

	//SETS EACH VARIABLE UPON CALLING FOR USE IN THE PROGRAM
	world := req.World
	var newHeight = req.EndY - req.StartY

	if req.ImageHeight == 16 && req.Turns == 100 {
		println(fmt.Sprintf("This is what is recieved on turn %v:", iteration))
		for i := 0; i < len(world); i++ {
			println()
			for j := 0; j < len(world[i]); j++ {
				print(fmt.Sprintf("(%v)", world[i][j]))
			}
		}
		println()

	}

	res.WorkSlice = make([][]byte, newHeight)
	for i := 0; i < newHeight; i++ {
		res.WorkSlice[i] = make([]byte, len(world[i]))
	}
	res.Worker = req.Worker

	//THE WORLD WILL BE UPDATED VIA THIS FUNCTION
	partWorld := calculateNextState(req.StartY, req.EndY, world)

	for i := 0; i < newHeight; i++ {
		for j := 0; j < len(world[i]); j++ {
			readWorld := partWorld[i][j]
			res.WorkSlice[i][j] = readWorld
		}
	}

	if req.ImageHeight == 16 && req.Turns == 100 {
		println(fmt.Sprintf("This is what is recieved on turn %v:", iteration))
		for i := 0; i < len(res.WorkSlice); i++ {
			println()
			for j := 0; j < len(res.WorkSlice[i]); j++ {
				print(fmt.Sprintf("(%v)", res.WorkSlice[i][j]))
			}
		}
		iteration += 1
		println()

	}

	return
}

func (s *GameOfLifeOperations) WorkerQuit(req stubs.Request, res *stubs.Response) (err error) {
	println("quit")
	workerQuit <- true
	return
}

//will be running separately on its own machine (AWS):
//the address will need to be found on the AWS machine, and flagged in the distributor
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&GameOfLifeOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	println(listener.Addr().String())
	go func() {
	serverLoop:
		for {
			print("loop")
			select {
			case <-workerQuit:
				println("Shutting down")
				listener.Close()
				break serverLoop

			}
		}
	}()
	rpc.Accept(listener)
	return
}

