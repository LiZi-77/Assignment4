package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	GroupThree "github.com/LiZi-77/PCPP-Assignment4/grpc"
	"google.golang.org/grpc"
)

// var port = flag.Int("port", 8000, "port") //port for the node default 8000

const INITPORT int32 = 8000

type STATE int32 //state of the node

const (
	RELEASED STATE = iota
	HELD
	REQUESTED
)

type node struct {
	GroupThree.UnimplementedGroup3Server
	requests int
	id       int32 //port
	clients  map[int32]GroupThree.Group3Client
	ctx      context.Context
	queue    []int32
	state    STATE
	mutex    *sync.Mutex
}

func main() {
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	var port = int32(arg1) + INITPORT

	logfile, err := os.OpenFile("log.server", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)

	if err != nil {
		log.Fatalf("failed opening file: %s", err)
	}
	defer logfile.Close()
	log.SetOutput(logfile)

	flag.Parse() //parse the flags

	ctx_, cancel := context.WithCancel(context.Background())
	defer cancel()

	//create a new node
	_n := &node{
		id:       port,
		clients:  make(map[int32]GroupThree.Group3Client),
		queue:    make([]int32, 0),
		ctx:      ctx_,
		state:    RELEASED,
		requests: 0,
	}

	list, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("Listening on port: %v\n", port)
	fmt.Println("Listening on port: ", port)

	// Create a new gRPC server
	_s := grpc.NewServer()
	GroupThree.RegisterGroup3Server(_s, _n)

	go func() {
		if err := _s.Serve(list); err != nil {
			log.Fatalf("failed to server %v\n", err)
		}
	}()

	//connect to all other nodes
	for i := 0; i < 3; i++ {
		nPort := int32(i) + INITPORT

		if nPort == _n.id {
			continue
		}

		var conn *grpc.ClientConn

		log.Printf("Dialing %v\n", nPort)
		fmt.Println("Dialing ", nPort)

		conn, err := grpc.Dial(fmt.Sprintf(":%v", nPort), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Dial failed %v\n", err)
		}

		defer conn.Close()
		log.Printf("Succes connecting to: %v\n", nPort)
		fmt.Println("Succes connecting to: ", nPort)

		//create a new client
		_c := GroupThree.NewGroup3Client(conn)
		_n.clients[nPort] = _c
	}

	log.Printf("%v is connected to %v other nodes\n", _n.id, len(_n.clients))
	fmt.Printf("%v is connected to %v other nodes\n", _n.id, len(_n.clients))

	// wait for user input
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		if scanner.Text() == "request" {
			if _n.state == RELEASED {
				_n.state = REQUESTED
				_n.requests++
				log.Printf("Node %v is requesting the token\n", _n.id)
				fmt.Printf("Node %v is requesting the token\n", _n.id)

				for _, client := range _n.clients {
					go func(client GroupThree.Group3Client) {
						_, err := client.RequestToken(_n.ctx, &GroupThree.RequestTokenRequest{
							Id: _n.id,
						})
						if err != nil {
							log.Printf("Error requesting token: %v\n", err)
						}
					}(client)
				}
			} else {
				log.Printf("Node %v is already holding the token\n", _n.id)
				fmt.Printf("Node %v is already holding the token\n", _n.id)
			}
		} else {
			log.Printf("Invalid command: %v\n", scanner.Text())
			fmt.Printf("Invalid command: %v\n", scanner.Text())
		}
	}

}

// request function to request access to the critical service
func (n *node) Request(ctx context.Context, req *GroupThree.Request) (*GroupThree.Ack, error) {
	log.Printf("%v recieved a request from %v.\n", n.id, req.Id)
	fmt.Println("Recieved a request")

	//checks if state is higher, if equal check id. (This is the hierarchy)
	if n.state == HELD || (n.state == REQUESTED && (n.id > req.Id)) {
		log.Printf("%v is queueing a request from %v\n", n.id, req.Id)
		fmt.Println("Queueing request")
		n.queue = append(n.queue, req.Id)
	} else {
		if n.state == REQUESTED {
			n.requests++
			n.clients[req.Id].Request(ctx, &GroupThree.Request{Id: n.id})
		}
		log.Printf("%v is sending a reply to %v\n", n.id, req.Id)
		fmt.Println("Sending reply")
		n.clients[req.Id].Reply(ctx, &GroupThree.Reply{})
	}
	reply := &GroupThree.Ack{}
	return reply, nil
}

// reply function to reply to a request
func (_n *node) Reply(ctx context.Context, req *GroupThree.Reply) (*GroupThree.AckReply, error) {

	_n.mutex.Lock()
	_n.requests--

	if _n.requests == 0 {
		_n.state = HELD
		log.Printf("%v is holding the token\n", _n.id)
		fmt.Println("Holding token")
	} else {
		log.Printf("%v is still waiting for %v replies\n", _n.id, _n.requests)
		fmt.Println("Waiting for replies")
	}
	_n.mutex.Unlock()

	reply := &GroupThree.AckReply{}
	return reply, nil
}

// critical function to simulate a critical service
func (_n *node) CriticalService() {
	log.Printf("%v is in the critical service\n", _n.id)
	fmt.Println("Critical service accessed")

	// wait for 5 seconds
	_n.state = HELD
	time.Sleep(5 * time.Second)
	_n.state = RELEASED

	log.Printf("%v is releasing the token\n", _n.id)
	fmt.Println("Releasing token")

	// reply to all queued requests
	_n.ReplyQueue()
}

// This is called when the node releases the token
func (_n *node) ReplyQueue() {
	reply := &GroupThree.Reply{}

	// reply to all queued requests
	for _, id := range _n.queue {
		_, err := _n.clients[id].Reply(_n.ctx, reply)

		if err != nil {
			log.Printf("Error replying to %v: %v\n", id, err)
		}
	}
	_n.queue = make([]int32, 0)
}
